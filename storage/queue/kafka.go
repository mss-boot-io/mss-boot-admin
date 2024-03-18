package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/mss-boot-io/mss-boot-admin/storage"
	"github.com/segmentio/kafka-go"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/13 20:01:18
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/13 20:01:18
 */

func NewKafka(brokers []string, partition int,
	writerConfig *kafka.Writer,
	readerConfig *kafka.ReaderConfig) *Kafka {
	return &Kafka{
		brokers:      brokers,
		partition:    partition,
		writerConfig: writerConfig,
		readerConfig: readerConfig,
		reader:       make(map[string]*kafka.Reader),
		writer:       make(map[string]*kafka.Writer),
	}
}

type Kafka struct {
	brokers      []string
	partition    int
	readerConfig *kafka.ReaderConfig
	writerConfig *kafka.Writer
	reader       map[string]*kafka.Reader
	writer       map[string]*kafka.Writer
	runReaders   []KafkaRunReader
	mux          sync.Mutex
}

type KafkaRunReader struct {
	Topic   string
	GroupID string
	Func    storage.ConsumerFunc
}

func (*Kafka) String() string {
	return "kafka"
}

func (e *Kafka) getWriter(topic string) *kafka.Writer {
	if e.writer == nil {
		e.writer = make(map[string]*kafka.Writer)
	}
	if w, ok := e.writer[topic]; ok {
		return w
	}
	var w *kafka.Writer
	if e.writerConfig != nil {
		*w = *e.writerConfig
		if len(e.brokers) > 0 {
			w.Addr = kafka.TCP(e.brokers...)
		}
		w.Topic = topic
		w.Balancer = &kafka.LeastBytes{}
		e.mux.Lock()
		e.writer[topic] = w
		e.mux.Unlock()
		return w
	}
	w = &kafka.Writer{
		Addr:     kafka.TCP(e.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return w
}

func (e *Kafka) getReader(topic string, groupID string) *kafka.Reader {
	if e.reader == nil {
		e.reader = make(map[string]*kafka.Reader)
	}
	if r, ok := e.reader[topic]; ok {
		return r
	}
	var r *kafka.Reader
	if e.readerConfig != nil {
		config := *e.readerConfig
		config.Topic = topic
		config.GroupID = groupID
		if len(e.brokers) > 0 {
			config.Brokers = e.brokers
		}
		r = kafka.NewReader(config)
		e.mux.Lock()
		e.reader[topic] = r
		e.mux.Unlock()
		return r
	}
	r = kafka.NewReader(kafka.ReaderConfig{
		Brokers:  e.brokers,
		Topic:    topic,
		GroupID:  groupID,
		MaxBytes: 10e6,
	})
	return r
}

func (e *Kafka) Append(message storage.Messager) error {
	rb, err := json.Marshal(message.GetValues())
	if err != nil {
		return err
	}
	w := e.getWriter(message.GetStream())
	ctx := message.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}
	return w.WriteMessages(ctx, kafka.Message{Value: rb})
}

func (e *Kafka) Register(topic, groupID string, f storage.ConsumerFunc) {
	if f == nil {
		panic("consumer func is nil")
	}
	if topic == "" {
		panic("topic is empty")
	}
	if e.runReaders == nil {
		e.runReaders = make([]KafkaRunReader, 0)
	}
	e.runReaders = append(e.runReaders, KafkaRunReader{
		Topic:   topic,
		GroupID: groupID,
		Func:    f,
	})
	r := e.getReader(topic, groupID)
	go func() {
		for {
			m, err := r.FetchMessage(context.Background())
			if err != nil {
				break
			}
			var data map[string]interface{}
			err = json.Unmarshal(m.Value, &data)
			if err != nil {
				continue
			}
			message := &Message{}
			message.SetValues(data)
			message.SetStream(topic)
			err = f(message)
		}
	}()
}

func (e *Kafka) Run() {
	for i := range e.runReaders {
		if e.runReaders[i].Func == nil {
			panic("consumer func is nil")
		}
		if e.runReaders[i].Topic == "" {
			panic("topic is empty")
		}
		r := e.getReader(e.runReaders[i].Topic, e.runReaders[i].GroupID)
		if r == nil {
			panic("reader is nil")
		}
		go func(topic, groupID string, reader *kafka.Reader, f storage.ConsumerFunc) {
			for {
				m, err := reader.FetchMessage(context.Background())
				if err != nil {
					break
				}
				var data map[string]interface{}
				err = json.Unmarshal(m.Value, &data)
				if err != nil {
					continue
				}
				message := &Message{}
				message.SetValues(data)
				message.SetID(groupID)
				message.SetStream(topic)
				err = f(message)
				if err != nil {
					slog.Error("consumer func error", slog.Any("error", err))
					continue
				}
				err = reader.CommitMessages(context.Background(), m)
				if err != nil {
					slog.Error("commit message error", slog.Any("error", err))
					continue
				}
			}
		}(e.runReaders[i].Topic, e.runReaders[i].GroupID, r, e.runReaders[i].Func)
	}
}

func (e *Kafka) Shutdown() {
	for _, r := range e.reader {
		if r != nil {
			err := r.Close()
			if err != nil {
				slog.Error("close reader error", slog.Any("error", err))
			}
		}
	}
	for _, w := range e.writer {
		if w != nil {
			err := w.Close()
			if err != nil {
				slog.Error("close writer error", slog.Any("error", err))
			}
		}
	}
}
