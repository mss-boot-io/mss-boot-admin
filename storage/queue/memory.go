package queue

import (
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/mss-boot-io/mss-boot-admin/storage"
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

func (m *Memory) Append(message storage.Messager) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	memoryMessage := new(Message)
	memoryMessage.SetID(message.GetID())
	memoryMessage.SetStream(message.GetStream())
	memoryMessage.SetValues(message.GetValues())

	v, ok := m.queue.Load(message.GetStream())

	if !ok {
		v = m.makeQueue()
		m.queue.Store(message.GetStream(), v)
	}

	var q memoryQueue
	switch v.(type) {
	case memoryQueue:
		q = v.(memoryQueue)
	default:
		q = m.makeQueue()
		m.queue.Store(message.GetStream(), q)
	}
	go func(gm storage.Messager, gq memoryQueue) {
		gm.SetID(uuid.New().String())
		gq <- gm
	}(memoryMessage, q)
	return nil
}

func (m *Memory) Register(name, _ string, f storage.ConsumerFunc) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	v, ok := m.queue.Load(name)
	if !ok {
		v = m.makeQueue()
		m.queue.Store(name, v)
	}
	var q memoryQueue
	switch v.(type) {
	case memoryQueue:
		q = v.(memoryQueue)
	default:
		q = m.makeQueue()
		m.queue.Store(name, q)
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
				err = nil
			}
		}
	}(q, f)
}

func (m *Memory) Run() {
	m.wait.Add(1)
	m.wait.Wait()
}

func (m *Memory) Shutdown() {
	m.wait.Done()
}
