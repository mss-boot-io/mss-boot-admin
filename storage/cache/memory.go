package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cast"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

type item struct {
	Value   string
	Expired time.Time
}

// NewMemory memory模式
func NewMemory(options ...Option) *Memory {
	o := DefaultOptions()
	for _, option := range options {
		option(&o)
	}
	return &Memory{
		items: new(sync.Map),
		opts:  o,
	}
}

type Memory struct {
	items *sync.Map
	mutex sync.RWMutex
	opts  Options
}

func (m *Memory) Initialize(tx *gorm.DB) error {
	return tx.Callback().Query().Replace("gorm:query", m.Query)
}

func (*Memory) Name() string {
	return "gorm:cache"
}

func (*Memory) String() string {
	return "memory"
}

func (m *Memory) connect() {
}

func (m *Memory) Get(_ context.Context, key string) (string, error) {
	e, err := m.getItem(key)
	if err != nil || e == nil {
		return "", err
	}
	return e.Value, nil
}

func (m *Memory) getItem(key string) (*item, error) {
	var err error
	i, ok := m.items.Load(key)
	if !ok {
		return nil, nil
	}
	switch i.(type) {
	case *item:
		e := i.(*item)
		if e.Expired.Before(time.Now()) {
			//过期
			_ = m.del(key)
			//过期后删除
			return nil, nil
		}
		return e, nil
	default:
		err = fmt.Errorf("value of %s type error", key)
		return nil, err
	}
}

func (m *Memory) Set(_ context.Context, key string, val any, expire time.Duration) error {
	s, err := cast.ToStringE(val)
	if err != nil {
		return err
	}
	e := &item{
		Value:   s,
		Expired: time.Now().Add(expire),
	}
	return m.setItem(key, e)
}

func (m *Memory) setItem(key string, item *item) error {
	m.items.Store(key, item)
	return nil
}

func (m *Memory) Del(_ context.Context, key string) error {
	return m.del(key)
}

func (m *Memory) del(key string) error {
	m.items.Delete(key)
	return nil
}

func (m *Memory) HashGet(_ context.Context, hk, key string) (string, error) {
	e, err := m.getItem(hk + key)
	if err != nil || e == nil {
		return "", err
	}
	return e.Value, err
}

func (m *Memory) HashDel(_ context.Context, hk, key string) error {
	return m.del(hk + key)
}

func (m *Memory) Increase(_ context.Context, key string) error {
	return m.calculate(key, 1)
}

func (m *Memory) Decrease(_ context.Context, key string) error {
	return m.calculate(key, -1)
}

func (m *Memory) calculate(key string, num int) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	e, err := m.getItem(key)
	if err != nil {
		return err
	}

	if e == nil {
		err = fmt.Errorf("%s not exist", key)
		return err
	}
	var n int
	n, err = cast.ToIntE(e.Value)
	if err != nil {
		return err
	}
	n += num
	e.Value = strconv.Itoa(n)
	return m.setItem(key, e)
}

func (m *Memory) Expire(_ context.Context, key string, dur time.Duration) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	e, err := m.getItem(key)
	if err != nil {
		return err
	}
	if e == nil {
		err = fmt.Errorf("%s not exist", key)
		return err
	}
	e.Expired = time.Now().Add(dur)
	return m.setItem(key, e)
}

func (m *Memory) Query(tx *gorm.DB) {
	ctx := tx.Statement.Context

	var (
		key    string
		hasKey bool
	)

	// 调用gorm的方法生产SQL
	callbacks.BuildQuerySQL(tx)

	// 是否有自定义key
	if key, hasKey = FromKey(ctx); !hasKey || !m.opts.HasKey(key) {
		key = m.generateKey(tx.Statement.SQL.String())
	}

	var useCache bool
	tag, hasTag := FromTag(ctx)
	tag = m.opts.QueryCachePrefix + tag
	if hasTag && m.opts.HasKey(tag) {
		useCache = true
	}

	// 查询缓存数据

	if useCache {
		if err := m.QueryCache(ctx, key, tx.Statement.Dest); err == nil {
			_ = m.SaveTagKey(ctx, tag, key)
			return
		}
	}

	// 查询数据库
	QueryDB(tx)

	if tx.Error != nil {
		return
	}
	if !useCache {
		return
	}

	// 写入缓存
	if err := m.SaveCache(ctx, key, tx.Statement.Dest, m.opts.QueryCacheDuration); err != nil {
		tx.Logger.Error(ctx, err.Error())
		return
	}
}

func (m *Memory) QueryCache(ctx context.Context, key string, dest any) error {
	s, err := m.Get(ctx, key)
	if err != nil {
		return err
	}
	if s == "" {
		return gorm.ErrRecordNotFound
	}

	switch dest.(type) {
	case *int64:
		dest = 0
	}
	return json.Unmarshal([]byte(s), dest)
}

func QueryDB(tx *gorm.DB) {
	if tx.Error != nil || tx.DryRun {
		return
	}
	rows, err := tx.Statement.ConnPool.QueryContext(tx.Statement.Context, tx.Statement.SQL.String(), tx.Statement.Vars...)
	if err != nil {
		_ = tx.AddError(err)
		return
	}

	defer func() {
		_ = tx.AddError(rows.Close())
	}()

	gorm.Scan(rows, tx, 0)
}

func (m *Memory) SaveCache(ctx context.Context, key string, dest any, ttl time.Duration) error {
	s, err := json.Marshal(dest)
	if err != nil {
		return err
	}
	return m.Set(ctx, key, string(s), ttl)
}

func (m *Memory) SaveTagKey(ctx context.Context, tag, key string) error {
	e, err := m.Get(ctx, tag)
	if err != nil || e == "" {
		// set tag
		return m.Set(ctx, tag, key, m.opts.QueryCacheDuration)
	}
	switch key {
	case "", "[]", "{}":
		return nil
	}
	for _, k := range strings.Split(e, ",") {
		if k == key {
			return nil
		}
	}
	e = strings.Join([]string{e, key}, ",")
	return m.Set(ctx, tag, e, m.opts.QueryCacheDuration)
}

func (m *Memory) RemoveFromTag(ctx context.Context, tag string) error {
	keys, err := m.Get(ctx, tag)
	if err != nil {
		return err
	}
	if keys == "" {
		return nil
	}
	for _, key := range strings.Split(keys, ",") {
		err = m.Del(ctx, key)
		if err != nil {
			return err
		}
	}
	_ = m.Del(ctx, tag)
	return nil
}

func (m *Memory) generateKey(key string) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key))

	return strconv.FormatUint(hash.Sum64(), 36)
}
