package consul

import (
	"errors"
	"io/fs"
	"log/slog"
	"strings"
	"time"

	consul "github.com/hashicorp/consul/api"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/source"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/28 22:26:27
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/28 22:26:27
 */

// Source is a consul source
type Source struct {
	opt  *source.Options
	name string
}

func (s *Source) GetExtend() source.Scheme {
	return s.opt.Extend
}

// Open a file for reading
func (s *Source) Open(_ string) (fs.File, error) {
	return nil, errors.New("method Get not implemented")
}

func (s *Source) ReadFile(name string) (rb []byte, err error) {
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return nil, err
	}
	key := strings.ReplaceAll(s.opt.Dir, "\\", "/")
	var kvPair *consul.KVPair
	for i := range source.Extends {
		kvPair, _, err = client.KV().Get(key+"/"+name+"."+string(source.Extends[i]), nil)
		if err != nil {
			break
		}
		if kvPair != nil {
			s.name = name
			s.opt.Extend = source.Extends[i]
			return kvPair.Value, nil
		}
	}
	return nil, err
}

func (s *Source) Watch(c source.Entity, unm func([]byte, any) error) error {
	client, err := consul.NewClient(consul.DefaultConfig())
	if err != nil {
		return err
	}
	key := strings.ReplaceAll(s.opt.Dir, "\\", "/")
	// 监听配置变化
	go func(sc *Source, cfg source.Entity, decoder func([]byte, any) error) {
		var pairs consul.KVPairs
		var meta *consul.QueryMeta
		params := &consul.QueryOptions{WaitIndex: 0, WaitTime: 10 * time.Minute}
		for {
			// 获取最新的配置变化
			pairs, meta, err = client.KV().List(key, params)
			if err != nil {
				slog.Error("Error watching for config changes", slog.Any("err", err))
				continue
			}
			if params.WaitIndex == 0 {
				params.WaitIndex = meta.LastIndex
				continue
			}
			if meta.LastIndex > params.WaitIndex {
				mapPairs := make(map[string]*consul.KVPair)
				for i := range pairs {
					mapPairs[pairs[i].Key] = pairs[i]
				}
				pair := mapPairs[key+"/"+sc.name+"."+string(sc.opt.Extend)]
				if pair != nil {
					if err = decoder(pair.Value, cfg); err != nil {
						slog.Error("Failed to decode config", slog.Any("error", err))
					}
				}
				pair = mapPairs[key+"/"+sc.name+"-"+pkg.GetStage()+"."+string(sc.opt.Extend)]
				if pair != nil {
					if err = decoder(pair.Value, cfg); err != nil {
						slog.Error("Failed to decode config", slog.Any("error", err))
					}
				}
				cfg.OnChange()
			}
			params.WaitIndex = meta.LastIndex
		}
	}(s, c, unm)
	return nil
}

func New(options ...source.Option) (*Source, error) {
	s := &Source{
		opt: source.DefaultOptions(),
	}
	for _, opt := range options {
		opt(s.opt)
	}
	return s, nil
}
