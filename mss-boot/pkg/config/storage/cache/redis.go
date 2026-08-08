package cache

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
)

const (
	generatedQueryCacheKeyNamespace           = "query:v3:"
	generatedQueryCacheGenerationKeyNamespace = "query-generation:v2:"
)

var (
	queryCacheProcessIdentityOnce sync.Once
	queryCacheProcessIdentity     [sha256.Size]byte
	queryCacheProcessIdentityOK   bool
	queryCacheConnPoolIdentities  sync.Map
	queryCacheConnPoolSequence    atomic.Uint64
)

// NewRedis redis模式
func NewRedis(client redis.UniversalClient, options *redis.UniversalOptions, opts ...Option) (*Redis, error) {
	o := DefaultOptions()
	for _, option := range opts {
		option(&o)
	}
	ownedClient := client == nil
	if ownedClient {
		client = redis.NewUniversalClient(options)
	}
	r := &Redis{
		UniversalClient: client,
		opts:            o,
	}
	err := r.connect()
	if err != nil {
		if ownedClient {
			if closeErr := client.Close(); closeErr != nil {
				return nil, errors.Join(err, fmt.Errorf("close query cache client after connect failure: %w", closeErr))
			}
		}
		return nil, err
	}
	return r, nil
}

// Redis cache implement
type Redis struct {
	redis.UniversalClient
	opts Options
}

func (r *Redis) Initialize(tx *gorm.DB) error {
	return tx.Callback().Query().Replace("gorm:query", r.Query)
}

func (*Redis) Name() string {
	return "gorm:cache"
}

func (*Redis) String() string {
	return "redis"
}

// connect connect test
func (r *Redis) connect() error {
	var err error
	_, err = r.Ping(context.TODO()).Result()
	return err
}

func (r *Redis) Query(tx *gorm.DB) {
	ctx := tx.Statement.Context
	if FromBypass(ctx) {
		QueryDB(tx)
		return
	}
	callbacks.BuildQuerySQL(tx)

	var useCache bool
	rawTag, hasTag := FromTag(ctx)
	if hasTag && r.opts.HasKey(rawTag) {
		useCache = true
	}
	if !useCache {
		QueryDB(tx)
		return
	}
	tag := r.opts.QueryCachePrefix + rawTag
	databaseIdentity, ok := queryCacheDatabaseIdentity(tx.Statement.ConnPool)
	if !ok {
		// A cache key without a stable database identity could alias another
		// database or tenant using the same Redis namespace. Fail open to SQL.
		QueryDB(tx)
		return
	}
	tagIdentity := queryCacheTagIdentity(tag)
	generationKey := queryCacheGenerationKey(r.opts.QueryCachePrefix, tagIdentity)
	generation, err := r.readQueryCacheGeneration(ctx, generationKey)
	if err != nil {
		// Redis is disposable acceleration. A generation that cannot be verified
		// must never authorize a cache hit or publication.
		QueryDB(tx)
		return
	}

	// A caller-provided key is an additional namespace, never a replacement
	// for the SQL and bound variables that determine the query result.
	keyNamespace, hasKey := FromKey(ctx)
	if !hasKey || !r.opts.HasKey(keyNamespace) {
		keyNamespace = ""
	}
	key, err := generateQueryCacheKey(
		r.opts.QueryCachePrefix,
		databaseIdentity,
		tagIdentity,
		generation,
		keyNamespace,
		tx.Statement.SQL.String(),
		tx.Statement.Vars,
	)
	if err != nil {
		// Unsupported driver values must not make the database query fail merely
		// because caching is enabled. Do not log the error: a driver.Valuer may
		// include sensitive parameter material in its error text.
		QueryDB(tx)
		return
	}

	if payload, cacheErr := r.queryCachePayload(ctx, key); cacheErr == nil {
		confirmedGeneration, generationErr := r.readQueryCacheGeneration(ctx, generationKey)
		if generationErr != nil {
			QueryDB(tx)
			return
		}
		if confirmedGeneration == generation {
			if decodeErr := decodeQueryCachePayload(payload, tx.Statement.Dest); decodeErr == nil {
				return
			}
		} else {
			// The cache entry was fetched while an invalidation advanced the tag.
			// Do not decode the stale payload. Re-anchor the database read to the
			// observed generation so a stable result may be published below.
			generation = confirmedGeneration
			key, err = generateQueryCacheKey(
				r.opts.QueryCachePrefix,
				databaseIdentity,
				tagIdentity,
				generation,
				keyNamespace,
				tx.Statement.SQL.String(),
				tx.Statement.Vars,
			)
			if err != nil {
				QueryDB(tx)
				return
			}
		}
	}

	QueryDB(tx)
	if tx.Error != nil {
		return
	}
	confirmedGeneration, err := r.readQueryCacheGeneration(ctx, generationKey)
	if err != nil || confirmedGeneration != generation {
		// The SQL result may have been read before a concurrent commit whose
		// invalidation is now visible. Return the database result to this caller,
		// but never resurrect it under an obsolete generation.
		return
	}
	ttl := r.opts.QueryCacheDuration
	if requestTTL, ok := FromExpiration(ctx); ok && requestTTL > 0 {
		ttl = requestTTL
	}

	if err := r.SaveCache(ctx, key, tx.Statement.Dest, ttl); err != nil {
		tx.Logger.Error(ctx, err.Error())
		return
	}
}

func (r *Redis) QueryCache(ctx context.Context, key string, dest any) error {
	payload, err := r.queryCachePayload(ctx, key)
	if err != nil {
		return err
	}
	return decodeQueryCachePayload(payload, dest)
}

func (r *Redis) queryCachePayload(ctx context.Context, key string) ([]byte, error) {
	s, err := r.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if s == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return []byte(s), nil
}

func decodeQueryCachePayload(payload []byte, dest any) error {
	switch dest.(type) {
	case *int64:
		dest = 0
	}
	return json.Unmarshal(payload, dest)
}

func (r *Redis) SaveCache(ctx context.Context, key string, dest any, ttl time.Duration) error {
	s, err := json.Marshal(dest)
	if err != nil {
		return err
	}
	return r.Set(ctx, key, string(s), ttl).Err()
}

func (r *Redis) SaveTagKey(ctx context.Context, tag, key string) error {
	return r.SAdd(ctx, tag, key).Err()
}

func (r *Redis) RemoveFromTag(ctx context.Context, tag string) error {
	generationKey := queryCacheGenerationKey(r.opts.QueryCachePrefix, queryCacheTagIdentity(tag))
	generation, err := newQueryCacheGenerationToken()
	if err != nil {
		return err
	}
	if err := r.Set(ctx, generationKey, generation, 0).Err(); err != nil {
		return err
	}
	// Older releases tracked every data key in a Redis set at tag. New
	// generation-keyed entries expire through their own TTL, so only the legacy
	// set itself needs best-effort compatibility cleanup. Keep this as a
	// separate one-key command so Redis Cluster never receives a cross-slot
	// operation.
	return r.Del(ctx, tag).Err()
}

// GetClient 暴露原生client
func (r *Redis) GetClient() redis.UniversalClient {
	return r
}

func queryCacheDatabaseIdentity(pool gorm.ConnPool) ([]byte, bool) {
	if pool == nil {
		return nil, false
	}
	// Transaction pools are short-lived and may expose uncommitted state. They
	// are not a stable database identity and must never participate in caching.
	if _, transactional := pool.(gorm.TxCommitter); transactional {
		return nil, false
	}

	var database *sql.DB
	switch typed := pool.(type) {
	case *sql.DB:
		database = typed
	case gorm.GetDBConnector:
		var err error
		database, err = typed.GetDBConn()
		if err != nil {
			return nil, false
		}
	default:
		return nil, false
	}
	if database == nil {
		return nil, false
	}

	queryCacheProcessIdentityOnce.Do(func() {
		_, err := rand.Read(queryCacheProcessIdentity[:])
		queryCacheProcessIdentityOK = err == nil
	})
	if !queryCacheProcessIdentityOK {
		return nil, false
	}

	actual, loaded := queryCacheConnPoolIdentities.Load(database)
	if !loaded {
		sequence := queryCacheConnPoolSequence.Add(1)
		actual, _ = queryCacheConnPoolIdentities.LoadOrStore(database, sequence)
	}
	poolIdentity, ok := actual.(uint64)
	if !ok {
		return nil, false
	}
	identity := make([]byte, 0, len(queryCacheProcessIdentity)+8)
	identity = append(identity, queryCacheProcessIdentity[:]...)
	var encodedSequence [8]byte
	binary.BigEndian.PutUint64(encodedSequence[:], poolIdentity)
	identity = append(identity, encodedSequence[:]...)
	return identity, true
}

func queryCacheTagIdentity(tag string) []byte {
	digest := sha256.Sum256([]byte(tag))
	return digest[:]
}

func queryCacheGenerationKey(cachePrefix string, tagIdentity []byte) string {
	return cachePrefix + generatedQueryCacheGenerationKeyNamespace + hex.EncodeToString(tagIdentity)
}

func (r *Redis) readQueryCacheGeneration(ctx context.Context, key string) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		generation, err := r.Get(ctx, key).Result()
		if err == nil {
			if !validQueryCacheGenerationToken(generation) {
				return "", errors.New("query cache generation token is invalid")
			}
			return generation, nil
		}
		if !errors.Is(err, redis.Nil) {
			return "", err
		}

		generation, err = newQueryCacheGenerationToken()
		if err != nil {
			return "", err
		}
		created, err := r.SetNX(ctx, key, generation, 0).Result()
		if err != nil {
			return "", err
		}
		if created {
			return generation, nil
		}
	}
	return "", errors.New("query cache generation changed while initializing")
}

func newQueryCacheGenerationToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func validQueryCacheGenerationToken(generation string) bool {
	if len(generation) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(generation)
	return err == nil && len(decoded) == 16
}

func generateQueryCacheKey(
	cachePrefix string,
	databaseIdentity []byte,
	tagIdentity []byte,
	generation string,
	namespace,
	sql string,
	vars []any,
) (string, error) {
	if len(databaseIdentity) == 0 {
		return "", errors.New("query cache database identity is empty")
	}
	if len(tagIdentity) == 0 {
		return "", errors.New("query cache tag identity is empty")
	}
	digest := sha256.New()
	writeQueryCacheKeyPart(digest, []byte("mss-gorm-query-cache-v3"))
	writeQueryCacheKeyPart(digest, databaseIdentity)
	writeQueryCacheKeyPart(digest, tagIdentity)
	writeQueryCacheKeyPart(digest, []byte(generation))
	writeQueryCacheKeyPart(digest, []byte(namespace))
	writeQueryCacheKeyPart(digest, []byte(sql))

	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(vars)))
	_, _ = digest.Write(count[:])
	for index, raw := range vars {
		value, err := driver.DefaultParameterConverter.ConvertValue(raw)
		if err != nil {
			return "", fmt.Errorf("convert query variable %d (%T): %w", index, raw, err)
		}
		if err := writeQueryCacheDriverValue(digest, value); err != nil {
			return "", fmt.Errorf("encode query variable %d (%T): %w", index, raw, err)
		}
	}

	return cachePrefix + generatedQueryCacheKeyNamespace + hex.EncodeToString(digest.Sum(nil)), nil
}

func writeQueryCacheDriverValue(digest hash.Hash, value driver.Value) error {
	switch value := value.(type) {
	case nil:
		_, _ = digest.Write([]byte{'n'})
	case int64:
		_, _ = digest.Write([]byte{'i'})
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], uint64(value))
		_, _ = digest.Write(encoded[:])
	case float64:
		_, _ = digest.Write([]byte{'f'})
		var encoded [8]byte
		binary.BigEndian.PutUint64(encoded[:], math.Float64bits(value))
		_, _ = digest.Write(encoded[:])
	case bool:
		if value {
			_, _ = digest.Write([]byte{'b', 1})
		} else {
			_, _ = digest.Write([]byte{'b', 0})
		}
	case []byte:
		_, _ = digest.Write([]byte{'x'})
		if value == nil {
			_, _ = digest.Write([]byte{0})
			return nil
		}
		_, _ = digest.Write([]byte{1})
		writeQueryCacheKeyPart(digest, value)
	case string:
		_, _ = digest.Write([]byte{'s'})
		writeQueryCacheKeyPart(digest, []byte(value))
	case time.Time:
		encoded, err := value.MarshalBinary()
		if err != nil {
			return err
		}
		_, _ = digest.Write([]byte{'t'})
		writeQueryCacheKeyPart(digest, encoded)
	default:
		return fmt.Errorf("unsupported driver value type %T", value)
	}
	return nil
}

func writeQueryCacheKeyPart(digest hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = digest.Write(length[:])
	_, _ = digest.Write(value)
}
