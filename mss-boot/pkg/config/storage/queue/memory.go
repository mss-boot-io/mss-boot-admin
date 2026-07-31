package queue

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
)

type memoryQueue chan storage.Messager

// NewMemory 内存模式
func NewMemory(poolNum uint) *Memory {
	return &Memory{
		queue:   new(sync.Map),
		PoolNum: poolNum,
	}
}

type Memory struct {
	queue   *sync.Map
	wait    sync.WaitGroup
	mutex   sync.RWMutex
	PoolNum uint
}

func (*Memory) String() string {
	return "memory"
}

func (m *Memory) makeQueue() memoryQueue {
	if m.PoolNum <= 0 {
		return make(memoryQueue)
	}
	return make(memoryQueue, m.PoolNum)
}

func (m *Memory) Append(opts ...storage.Option) error {
	o := storage.SetOptions(opts...)
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	memoryMessage := new(Message)
	memoryMessage.SetID(o.Message.GetID())
	memoryMessage.SetStream(o.Message.GetStream())
	rb, err := json.Marshal(o.Message.GetValues())
	if err != nil {
		return err
	}
	data := make(map[string]any)
	_ = json.Unmarshal(rb, &data)
	memoryMessage.SetValues(data)

	v, ok := m.queue.Load(o.Message.GetStream())

	if !ok {
		v = m.makeQueue()
		m.queue.Store(o.Message.GetStream(), v)
	}

	var q memoryQueue
	switch v := v.(type) {
	case memoryQueue:
		q = v
	default:
		q = m.makeQueue()
		m.queue.Store(o.Message.GetStream(), q)
	}
	go func(gm storage.Messager, gq memoryQueue) {
		gm.SetID(uuid.New().String())
		gq <- gm
	}(memoryMessage, q)
	return nil
}

func (m *Memory) Register(opts ...storage.Option) {
	o := storage.SetOptions(opts...)
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	v, ok := m.queue.Load(o.Topic)
	if !ok {
		v = m.makeQueue()
		m.queue.Store(o.Topic, v)
	}
	var q memoryQueue
	switch v := v.(type) {
	case memoryQueue:
		q = v
	default:
		q = m.makeQueue()
		m.queue.Store(o.Topic, q)
	}
	go func(out memoryQueue, gf storage.ConsumerFunc) {
		var err error
		for message := range q {
			err = gf(message)
			if err != nil {
				if message.GetErrorCount() < 3 {
					message.SetErrorCount(message.GetErrorCount() + 1)
					// 每次间隔时长放大
					i := time.Second * time.Duration(message.GetErrorCount())
					time.Sleep(i)
					out <- message
				}
			}
		}
	}(q, o.F)
}

func (m *Memory) Run(context.Context) {
	m.wait.Add(1)
	m.wait.Wait()
}

func (m *Memory) Shutdown() {
	m.wait.Done()
}
