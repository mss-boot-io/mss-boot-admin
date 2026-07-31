package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"sync"

	"github.com/IBM/sarama"

	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/13 20:01:18
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/13 20:01:18
 */

type ConsumerGroupHandler interface {
	sarama.ConsumerGroupHandler
	SetConsumerFunc(f storage.ConsumerFunc)
}

func NewKafka(brokers []string, c *sarama.Config, h ConsumerGroupHandler, provider string) (k *Kafka, err error) {
	k = &Kafka{brokers: brokers, config: c, consumerGroupHandler: h, provider: provider}
	return
}

type ConsumerRegister struct {
	Topic     string
	GroupID   string
	Partition int
	Func      ConsumerGroupHandler
}

type Kafka struct {
	mux                  sync.Mutex
	consumers            map[*ConsumerRegister]sarama.ConsumerGroup
	brokers              []string
	config               *sarama.Config
	producer             sarama.SyncProducer
	asyncProducer        sarama.AsyncProducer
	consumerGroupHandler sarama.ConsumerGroupHandler
	provider             string
}

type KafkaRunReader struct {
	Topic   string
	GroupID string
	Func    storage.ConsumerFunc
}

func (*Kafka) String() string {
	return "kafka"
}

func (e *Kafka) Append(opts ...storage.Option) error {
	o := storage.SetOptions(opts...)
	for _, opt := range opts {
		opt(o)
	}
	if e.config != nil && e.producer == nil {
		var err error
		c := *e.config
		if o.KafkaConfig != nil {
			c.Producer = o.KafkaConfig.Producer
		}
		e.producer, err = sarama.NewSyncProducer(e.brokers, &c)
		if err != nil {
			return err
		}
	}
	rb, err := json.Marshal(o.Message.GetValues())
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: o.Message.GetStream(),
		Key:   sarama.StringEncoder(o.Message.GetID()),
		Value: sarama.ByteEncoder(rb),
	}
	_, _, err = e.producer.SendMessage(msg)
	return err
}

func (e *Kafka) Register(opts ...storage.Option) {
	o := storage.SetOptions(opts...)
	if o.F == nil {
		slog.Error("consumer func is nil")
		os.Exit(-1)
	}
	if o.Topic == "" {
		slog.Error("topic is empty")
		os.Exit(-1)
	}
	if o.PartitionAssignmentStrategy != nil && o.Partition >= 0 {
		e.config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{o.PartitionAssignmentStrategy}
		e.config.Consumer.Group.Member.UserData = []byte{byte(o.Partition)}
	}
	c := *e.config
	if o.KafkaConfig != nil {
		c.Consumer = o.KafkaConfig.Consumer
	}
	consumer, err := sarama.NewConsumerGroup(e.brokers, o.GroupID, &c)
	if err != nil {
		slog.Error("create consumer group error", slog.Any("error", err))
		os.Exit(-1)
	}
	// copy the consumer to use it in the handler
	cf, ok := reflect.New(reflect.TypeOf(e.consumerGroupHandler).Elem()).Interface().(ConsumerGroupHandler)
	if !ok {
		slog.Error("type assertion error")
		os.Exit(-1)
	}
	cf.SetConsumerFunc(o.F)

	if e.consumers == nil {
		e.consumers = make(map[*ConsumerRegister]sarama.ConsumerGroup)
	}
	e.mux.Lock()
	e.consumers[&ConsumerRegister{Topic: o.Topic, GroupID: o.GroupID, Func: cf}] = consumer
	e.mux.Unlock()
}

func (e *Kafka) Run(ctx context.Context) {
	for r, c := range e.consumers {
		go func(r *ConsumerRegister, c sarama.ConsumerGroup) {
			for {
				err := c.Consume(ctx, []string{r.Topic}, r.Func)
				if err != nil {
					slog.Error("consume error", slog.Any("error", err))
				}
			}
		}(r, c)
	}
}

func (e *Kafka) Shutdown() {
	for _, c := range e.consumers {
		if err := c.Close(); err != nil {
			slog.Error("close consumer error", slog.Any("error", err))
		}
	}
}

type MessageHandler struct {
	f storage.ConsumerFunc
}

func (h *MessageHandler) Setup(s sarama.ConsumerGroupSession) error {
	slog.Debug("Partition allocation -", slog.Any("claims", s.Claims()))
	return nil
}

func (h *MessageHandler) Cleanup(sarama.ConsumerGroupSession) error {
	slog.Debug("Consumer group clean up initiated")
	return nil
}
func (h *MessageHandler) ConsumeClaim(s sarama.ConsumerGroupSession, c sarama.ConsumerGroupClaim) error {
	if h.f == nil {
		return errors.New("consumer func is nil")
	}
	var data map[string]any
	for msg := range c.Messages() {
		data = make(map[string]any)
		slog.Debug(fmt.Sprintf("Message topic:%q partition:%d offset:%d\n", msg.Topic, msg.Partition, msg.Offset))
		slog.Debug("Message content", slog.String("value", string(msg.Value)))
		s.MarkMessage(msg, "")
		message := &Message{}
		message.SetID(string(msg.Key))
		message.SetStream(msg.Topic)
		err := json.Unmarshal(msg.Value, &data)
		if err != nil {
			slog.Error("unmarshal message error", slog.Any("error", err))
			return err
		}
		message.SetValues(data)
		err = h.f(message)
		if err != nil {
			slog.Error("consumer func error", slog.Any("error", err))
			return err
		}
	}
	return nil
}

func (h *MessageHandler) SetConsumerFunc(f storage.ConsumerFunc) {
	h.f = f
}
