package queue

import (
	"context"
	"sync"

	"github.com/mss-boot-io/redisqueue/v2"

	"github.com/mss-boot-io/mss-boot-admin/storage"
)

type Message struct {
	redisqueue.Message
	ErrorCount int
	mux        sync.RWMutex
	ctx        context.Context
}

func (m *Message) GetID() string {
	return m.ID
}

func (m *Message) GetStream() string {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.Stream
}

func (m *Message) GetValues() map[string]interface{} {
	m.mux.Lock()
	defer m.mux.Unlock()
	data := make(map[string]interface{})
	for k, v := range m.Values {
		data[k] = v
	}
	data["__id"] = m.ID
	data["__steam"] = m.Stream
	return data
}

func (m *Message) SetID(id string) {
	m.ID = id
}

func (m *Message) SetStream(stream string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.Stream = stream
}

func (m *Message) SetValues(values map[string]interface{}) {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.ID, _ = values["__id"].(string)
	m.Stream, _ = values["__steam"].(string)
	delete(values, "__id")
	delete(values, "__steam")
	m.Values = values
}

func (m *Message) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Message) GetPrefix() (prefix string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.Values == nil {
		return
	}
	v, _ := m.Values[storage.PrefixKey]
	prefix, _ = v.(string)
	return
}

func (m *Message) SetPrefix(prefix string) {
	m.mux.Lock()
	defer m.mux.Unlock()
	if m.Values == nil {
		m.Values = make(map[string]interface{})
	}
	m.Values[storage.PrefixKey] = prefix
}

func (m *Message) SetErrorCount(count int) {
	m.ErrorCount = count
}

func (m *Message) GetErrorCount() int {
	return m.ErrorCount
}

func (m *Message) GetContext() context.Context {
	return m.ctx
}
