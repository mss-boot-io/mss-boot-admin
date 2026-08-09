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

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
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
		go e.runConsumer(ctx, r, c)
	}
}

func (e *Kafka) runConsumer(ctx context.Context, r *ConsumerRegister, c sarama.ConsumerGroup) {
	for ctx.Err() == nil {
		err := c.Consume(ctx, []string{r.Topic}, r.Func)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, sarama.ErrClosedConsumerGroup) {
			return
		}
		if err != nil {
			slog.Error("consume error", slog.Any("error", err))
		}
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
	for {
		if s.Context().Err() != nil {
			return nil
		}
		select {
		case <-s.Context().Done():
			return nil
		case msg, ok := <-c.Messages():
			if !ok {
				return nil
			}
			if msg == nil {
				return errors.New("consumer message is nil")
			}
			if s.Context().Err() != nil {
				return nil
			}

			slog.Debug("kafka message received",
				slog.String("topic", msg.Topic),
				slog.Int("partition", int(msg.Partition)),
				slog.Int64("offset", msg.Offset),
			)
			data := make(map[string]any)
			if err := json.Unmarshal(msg.Value, &data); err != nil {
				return fmt.Errorf("decode Kafka message: %w", err)
			}
			message := &Message{}
			message.SetID(string(msg.Key))
			message.SetStream(msg.Topic)
			message.SetValues(data)
			message.SetContext(s.Context())
			if err := h.f(message); err != nil {
				return fmt.Errorf("handle Kafka message: %w", err)
			}
			// A canceled session cannot safely publish a new offset. Leaving the
			// message unmarked keeps it eligible for redelivery in a later session.
			if s.Context().Err() != nil {
				return nil
			}
			s.MarkMessage(msg, "")
		}
	}
}

func (h *MessageHandler) SetConsumerFunc(f storage.ConsumerFunc) {
	h.f = f
}
