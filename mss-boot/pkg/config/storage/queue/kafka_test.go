package queue

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

type kafkaTestSession struct {
	ctx context.Context

	mu    sync.Mutex
	marks []kafkaTestMark
}

type kafkaTestMark struct {
	method     string
	message    *sarama.ConsumerMessage
	topic      string
	partition  int32
	nextOffset int64
	metadata   string
}

func (s *kafkaTestSession) Claims() map[string][]int32 { return nil }
func (s *kafkaTestSession) MemberID() string           { return "test-member" }
func (s *kafkaTestSession) GenerationID() int32        { return 1 }
func (s *kafkaTestSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marks = append(s.marks, kafkaTestMark{
		method:     "MarkOffset",
		topic:      topic,
		partition:  partition,
		nextOffset: offset,
		metadata:   metadata,
	})
}
func (s *kafkaTestSession) Commit()                                  {}
func (s *kafkaTestSession) ResetOffset(string, int32, int64, string) {}
func (s *kafkaTestSession) Context() context.Context                 { return s.ctx }
func (s *kafkaTestSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marks = append(s.marks, kafkaTestMark{
		method:     "MarkMessage",
		message:    msg,
		topic:      msg.Topic,
		partition:  msg.Partition,
		nextOffset: msg.Offset + 1,
		metadata:   metadata,
	})
}

func (s *kafkaTestSession) recordedMarks() []kafkaTestMark {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]kafkaTestMark(nil), s.marks...)
}

type kafkaTestClaim struct {
	messages     <-chan *sarama.ConsumerMessage
	messageCalls *atomic.Int32
}

func (c kafkaTestClaim) Topic() string              { return "events" }
func (c kafkaTestClaim) Partition() int32           { return 2 }
func (c kafkaTestClaim) InitialOffset() int64       { return 0 }
func (c kafkaTestClaim) HighWaterMarkOffset() int64 { return 0 }
func (c kafkaTestClaim) Messages() <-chan *sarama.ConsumerMessage {
	if c.messageCalls != nil {
		c.messageCalls.Add(1)
	}
	return c.messages
}

func kafkaClaim(messages ...*sarama.ConsumerMessage) sarama.ConsumerGroupClaim {
	stream := make(chan *sarama.ConsumerMessage, len(messages))
	for _, message := range messages {
		stream <- message
	}
	close(stream)
	return kafkaTestClaim{messages: stream}
}

func kafkaMessage(offset int64, value string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Topic:     "events",
		Partition: 2,
		Offset:    offset,
		Key:       []byte("event-id"),
		Value:     []byte(value),
	}
}

func TestKafkaDoesNotMarkOnDecodeFailure(t *testing.T) {
	session := &kafkaTestSession{ctx: context.Background()}
	var handled atomic.Int32
	handler := &MessageHandler{f: func(storage.Messager) error {
		handled.Add(1)
		return nil
	}}

	err := handler.ConsumeClaim(session, kafkaClaim(
		kafkaMessage(7, `{invalid`),
		kafkaMessage(8, `{"must":"not-run"}`),
	))
	if err == nil {
		t.Fatal("ConsumeClaim() error = nil, want decode error")
	}
	if handled.Load() != 0 {
		t.Fatalf("handler calls = %d, want 0", handled.Load())
	}
	if marks := session.recordedMarks(); len(marks) != 0 {
		t.Fatalf("marks = %d, want 0", len(marks))
	}
}

func TestKafkaDoesNotMarkOnHandlerFailure(t *testing.T) {
	handlerErr := errors.New("handler failed")
	session := &kafkaTestSession{ctx: context.Background()}
	var handled atomic.Int32
	handler := &MessageHandler{f: func(storage.Messager) error {
		if handled.Add(1) == 2 {
			return handlerErr
		}
		return nil
	}}

	first := kafkaMessage(8, `{"sequence":1}`)
	err := handler.ConsumeClaim(session, kafkaClaim(
		first,
		kafkaMessage(9, `{"sequence":2}`),
		kafkaMessage(10, `{"sequence":3}`),
	))
	if !errors.Is(err, handlerErr) {
		t.Fatalf("ConsumeClaim() error = %v, want wrapped handler error", err)
	}
	if handled.Load() != 2 {
		t.Fatalf("handler calls = %d, want 2 so the queued third message cannot skip the failure", handled.Load())
	}
	marks := session.recordedMarks()
	if len(marks) != 1 || marks[0].message != first {
		t.Fatalf("marks = %#v, want only the successful message before the failure", marks)
	}
}

func TestKafkaMarksOnceAfterHandlerSuccess(t *testing.T) {
	session := &kafkaTestSession{ctx: context.Background()}
	first := kafkaMessage(10, `{"kind":"policy-revision"}`)
	second := kafkaMessage(11, `{"kind":"policy-revision"}`)
	var handled atomic.Int32
	handler := &MessageHandler{f: func(message storage.Messager) error {
		calls := handled.Add(1)
		if marks := session.recordedMarks(); len(marks) != int(calls-1) {
			t.Fatalf("marks visible inside handler %d = %d, want %d", calls, len(marks), calls-1)
		}
		if message.GetID() != "event-id" || message.GetStream() != "events" {
			t.Fatalf("decoded identity = (%q, %q)", message.GetID(), message.GetStream())
		}
		if message.GetValues()["kind"] != "policy-revision" {
			t.Fatalf("decoded values = %#v", message.GetValues())
		}
		if message.GetContext() != session.ctx {
			t.Fatal("message did not inherit the consumer session context")
		}
		return nil
	}}

	if err := handler.ConsumeClaim(session, kafkaClaim(first, second)); err != nil {
		t.Fatalf("ConsumeClaim() error = %v", err)
	}
	if handled.Load() != 2 {
		t.Fatalf("handler calls = %d, want 2", handled.Load())
	}
	marks := session.recordedMarks()
	if len(marks) != 2 || marks[0].message != first || marks[1].message != second {
		t.Fatalf("marks = %#v, want both successful inputs once and in order", marks)
	}
	for index, mark := range marks {
		wantOffset := int64(11 + index)
		if mark.method != "MarkMessage" || mark.topic != "events" || mark.partition != 2 || mark.nextOffset != wantOffset || mark.metadata != "" {
			t.Fatalf("mark %d = %#v, want MarkMessage for next offset %d", index, mark, wantOffset)
		}
	}
}

func TestKafkaCancellationLeavesUnfinishedUnmarked(t *testing.T) {
	t.Run("already canceled buffered message", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		session := &kafkaTestSession{ctx: ctx}
		stream := make(chan *sarama.ConsumerMessage, 1)
		stream <- kafkaMessage(11, `{"kind":"canceled"}`)
		close(stream)
		var messageCalls atomic.Int32
		var handled atomic.Int32
		handler := &MessageHandler{f: func(storage.Messager) error {
			handled.Add(1)
			return nil
		}}
		if err := handler.ConsumeClaim(session, kafkaTestClaim{messages: stream, messageCalls: &messageCalls}); err != nil {
			t.Fatalf("ConsumeClaim() error = %v", err)
		}
		if messageCalls.Load() != 0 {
			t.Fatalf("Messages calls = %d, want 0 after the canceled-session preflight", messageCalls.Load())
		}
		if handled.Load() != 0 || len(session.recordedMarks()) != 0 {
			t.Fatalf("canceled session handled=%d marks=%d, want 0/0", handled.Load(), len(session.recordedMarks()))
		}
	})

	t.Run("idle open claim", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		session := &kafkaTestSession{ctx: ctx}
		stream := make(chan *sarama.ConsumerMessage)
		done := make(chan error, 1)
		go func() {
			done <- (&MessageHandler{f: func(storage.Messager) error { return nil }}).ConsumeClaim(
				session,
				kafkaTestClaim{messages: stream},
			)
		}()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("ConsumeClaim() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("idle claim did not stop after cancellation")
		}
	})

	t.Run("in flight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		session := &kafkaTestSession{ctx: ctx}
		started := make(chan struct{})
		handler := &MessageHandler{f: func(message storage.Messager) error {
			close(started)
			<-message.GetContext().Done()
			return message.GetContext().Err()
		}}
		done := make(chan error, 1)
		go func() {
			done <- handler.ConsumeClaim(session, kafkaClaim(kafkaMessage(12, `{"kind":"slow"}`)))
		}()

		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("handler did not start")
		}
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("ConsumeClaim() cancellation error = %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("ConsumeClaim() did not stop after session cancellation")
		}
		if marks := session.recordedMarks(); len(marks) != 0 {
			t.Fatalf("marks = %d, want 0", len(marks))
		}
	})

	t.Run("handler completes after cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		session := &kafkaTestSession{ctx: ctx}
		started := make(chan struct{})
		release := make(chan struct{})
		handler := &MessageHandler{f: func(storage.Messager) error {
			close(started)
			<-release
			return nil
		}}
		done := make(chan error, 1)
		go func() {
			done <- handler.ConsumeClaim(session, kafkaClaim(kafkaMessage(13, `{"kind":"completed"}`)))
		}()

		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("handler did not start")
		}
		cancel()
		close(release)
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("ConsumeClaim() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("ConsumeClaim() did not stop after the canceled handler completed")
		}
		if marks := session.recordedMarks(); len(marks) != 0 {
			t.Fatalf("marks = %d, want 0 so completed side effects remain eligible for redelivery", len(marks))
		}
	})
}

type kafkaTestConsumerGroup struct {
	consumeCalls atomic.Int32
	errorsCalls  atomic.Int32
	closeCalls   atomic.Int32
	started      chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
	closed       chan struct{}
	errors       <-chan error
	consume      func(context.Context, int32) error
}

func (g *kafkaTestConsumerGroup) Consume(ctx context.Context, _ []string, _ sarama.ConsumerGroupHandler) error {
	calls := g.consumeCalls.Add(1)
	if g.started != nil {
		g.startOnce.Do(func() { close(g.started) })
	}
	if g.consume != nil {
		return g.consume(ctx, calls)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (g *kafkaTestConsumerGroup) Errors() <-chan error {
	g.errorsCalls.Add(1)
	return g.errors
}
func (*kafkaTestConsumerGroup) Pause(map[string][]int32)  {}
func (*kafkaTestConsumerGroup) Resume(map[string][]int32) {}
func (*kafkaTestConsumerGroup) PauseAll()                 {}
func (*kafkaTestConsumerGroup) ResumeAll()                {}
func (g *kafkaTestConsumerGroup) Close() error {
	g.closeCalls.Add(1)
	if g.closed != nil {
		g.closeOnce.Do(func() { close(g.closed) })
	}
	return nil
}

type kafkaTestProducer struct {
	sendCalls  atomic.Int32
	closeCalls atomic.Int32

	mu       sync.Mutex
	messages []*sarama.ProducerMessage

	started     chan struct{}
	startOnce   sync.Once
	sendRelease <-chan struct{}
	sendErr     error
	closeErr    error
}

func (p *kafkaTestProducer) SendMessage(message *sarama.ProducerMessage) (int32, int64, error) {
	p.sendCalls.Add(1)
	p.mu.Lock()
	p.messages = append(p.messages, message)
	p.mu.Unlock()
	if p.started != nil {
		p.startOnce.Do(func() { close(p.started) })
	}
	if p.sendRelease != nil {
		<-p.sendRelease
	}
	return 0, int64(p.sendCalls.Load()), p.sendErr
}

func (p *kafkaTestProducer) SendMessages(messages []*sarama.ProducerMessage) error {
	for _, message := range messages {
		if _, _, err := p.SendMessage(message); err != nil {
			return err
		}
	}
	return nil
}

func (p *kafkaTestProducer) Close() error {
	p.closeCalls.Add(1)
	return p.closeErr
}

func (*kafkaTestProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }
func (*kafkaTestProducer) IsTransactional() bool                   { return false }
func (*kafkaTestProducer) BeginTxn() error                         { return nil }
func (*kafkaTestProducer) CommitTxn() error                        { return nil }
func (*kafkaTestProducer) AbortTxn() error                         { return nil }
func (*kafkaTestProducer) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (*kafkaTestProducer) AddOffsetsToTxnWithGroupMetadata(
	map[string][]*sarama.PartitionOffsetMetadata,
	*sarama.ConsumerGroupMetadata,
) error {
	return nil
}
func (*kafkaTestProducer) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error {
	return nil
}
func (*kafkaTestProducer) AddMessageToTxnWithGroupMetadata(
	*sarama.ConsumerMessage,
	*sarama.ConsumerGroupMetadata,
	*string,
) error {
	return nil
}

func (p *kafkaTestProducer) recordedMessages() []*sarama.ProducerMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*sarama.ProducerMessage(nil), p.messages...)
}

type kafkaTestValueHandler struct{}

func (kafkaTestValueHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (kafkaTestValueHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (kafkaTestValueHandler) ConsumeClaim(sarama.ConsumerGroupSession, sarama.ConsumerGroupClaim) error {
	return nil
}
func (kafkaTestValueHandler) SetConsumerFunc(storage.ConsumerFunc) {}

func newKafkaTestQueue(
	t *testing.T,
	producer *kafkaTestProducer,
	consumerFactory kafkaConsumerFactory,
) *Kafka {
	t.Helper()
	if producer == nil {
		producer = &kafkaTestProducer{}
	}
	if consumerFactory == nil {
		consumerFactory = func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
			return &kafkaTestConsumerGroup{}, nil
		}
	}
	queue, err := newKafka(
		[]string{"broker.test:9092"},
		sarama.NewConfig(),
		&MessageHandler{},
		"test",
		func([]string, *sarama.Config) (sarama.SyncProducer, error) {
			return producer, nil
		},
		consumerFactory,
	)
	if err != nil {
		t.Fatalf("newKafka() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := queue.Close(ctx); err != nil {
			t.Errorf("Kafka.Close() cleanup error = %v", err)
		}
	})
	return queue
}

func kafkaTestMessage(id, stream string) *Message {
	message := &Message{}
	message.SetID(id)
	message.SetStream(stream)
	message.SetValues(map[string]any{"kind": "test"})
	return message
}

func waitKafkaTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitKafkaTestError(t *testing.T, result <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func registerKafkaTestConsumer(t *testing.T, queue *Kafka, topic, group string) {
	t.Helper()
	err := queue.RegisterContext(
		context.Background(),
		storage.WithTopic(topic),
		storage.WithGroupID(group),
		storage.WithConsumerFunc(func(storage.Messager) error { return nil }),
	)
	if err != nil {
		t.Fatalf("RegisterContext() error = %v", err)
	}
}

func TestKafkaConstructionOwnsOneStrictProducer(t *testing.T) {
	configuration := sarama.NewConfig()
	configuration.Producer.Return.Errors = false
	configuration.Producer.Return.Successes = false
	configuration.Consumer.Return.Errors = false
	producer := &kafkaTestProducer{}
	var producerFactoryCalls atomic.Int32

	queue, err := newKafka(
		[]string{" broker.test:9092 ", "broker.test:9092"},
		configuration,
		&MessageHandler{},
		"test",
		func(brokers []string, owned *sarama.Config) (sarama.SyncProducer, error) {
			producerFactoryCalls.Add(1)
			if len(brokers) != 1 || brokers[0] != "broker.test:9092" {
				t.Fatalf("owned brokers = %#v", brokers)
			}
			if !owned.Producer.Return.Errors || !owned.Producer.Return.Successes || !owned.Consumer.Return.Errors {
				t.Fatalf("owned return channels = producer errors:%t successes:%t consumer errors:%t, want all true",
					owned.Producer.Return.Errors,
					owned.Producer.Return.Successes,
					owned.Consumer.Return.Errors,
				)
			}
			return producer, nil
		},
		func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
			return &kafkaTestConsumerGroup{}, nil
		},
	)
	if err != nil {
		t.Fatalf("newKafka() error = %v", err)
	}
	if producerFactoryCalls.Load() != 1 {
		t.Fatalf("producer factory calls = %d, want 1", producerFactoryCalls.Load())
	}
	if configuration.Producer.Return.Errors || configuration.Producer.Return.Successes || configuration.Consumer.Return.Errors {
		t.Fatal("newKafka() mutated the caller-owned configuration")
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if producer.closeCalls.Load() != 1 {
		t.Fatalf("producer Close calls = %d, want 1", producer.closeCalls.Load())
	}
}

func TestKafkaConstructionReturnsValidationAndProducerErrors(t *testing.T) {
	producerErr := errors.New("producer unavailable")
	validProducerFactory := func([]string, *sarama.Config) (sarama.SyncProducer, error) {
		return &kafkaTestProducer{}, nil
	}
	consumerFactory := func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		return &kafkaTestConsumerGroup{}, nil
	}

	t.Run("brokers required", func(t *testing.T) {
		_, err := newKafka(nil, sarama.NewConfig(), &MessageHandler{}, "test", validProducerFactory, consumerFactory)
		if err == nil {
			t.Fatal("newKafka() error = nil, want broker validation error")
		}
	})
	t.Run("configuration required", func(t *testing.T) {
		_, err := newKafka([]string{"broker.test:9092"}, nil, &MessageHandler{}, "test", validProducerFactory, consumerFactory)
		if err == nil {
			t.Fatal("newKafka() error = nil, want configuration validation error")
		}
	})
	t.Run("automatic commit required", func(t *testing.T) {
		configuration := sarama.NewConfig()
		configuration.Consumer.Offsets.AutoCommit.Enable = false
		_, err := newKafka([]string{"broker.test:9092"}, configuration, &MessageHandler{}, "test", validProducerFactory, consumerFactory)
		if err == nil {
			t.Fatal("newKafka() error = nil, want automatic-commit validation error")
		}
	})
	t.Run("pointer handler required", func(t *testing.T) {
		_, err := newKafka([]string{"broker.test:9092"}, sarama.NewConfig(), kafkaTestValueHandler{}, "test", validProducerFactory, consumerFactory)
		if err == nil {
			t.Fatal("newKafka() error = nil, want handler prototype validation error")
		}
	})
	t.Run("producer construction failure", func(t *testing.T) {
		_, err := newKafka(
			[]string{"broker.test:9092"},
			sarama.NewConfig(),
			&MessageHandler{},
			"test",
			func([]string, *sarama.Config) (sarama.SyncProducer, error) { return nil, producerErr },
			consumerFactory,
		)
		if !errors.Is(err, producerErr) {
			t.Fatalf("newKafka() error = %v, want wrapped producer error", err)
		}
	})
	t.Run("nil producer rejected", func(t *testing.T) {
		_, err := newKafka(
			[]string{"broker.test:9092"},
			sarama.NewConfig(),
			&MessageHandler{},
			"test",
			func([]string, *sarama.Config) (sarama.SyncProducer, error) { return nil, nil },
			consumerFactory,
		)
		if err == nil {
			t.Fatal("newKafka() error = nil, want nil producer error")
		}
	})
}

func TestKafkaRegisterReturnsValidationAndConstructionErrors(t *testing.T) {
	consumerErr := errors.New("consumer unavailable")
	var consumerFactoryCalls atomic.Int32
	queue := newKafkaTestQueue(t, nil, func(_ []string, group string, _ *sarama.Config) (sarama.ConsumerGroup, error) {
		consumerFactoryCalls.Add(1)
		switch group {
		case "factory-error":
			return nil, consumerErr
		case "factory-nil":
			return nil, nil
		default:
			return &kafkaTestConsumerGroup{}, nil
		}
	})
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	disabledAutoCommit := sarama.NewConfig()
	disabledAutoCommit.Consumer.Offsets.AutoCommit.Enable = false
	consumer := storage.WithConsumerFunc(func(storage.Messager) error { return nil })

	tests := []struct {
		name   string
		ctx    context.Context
		opts   []storage.Option
		wantIs error
	}{
		{name: "nil context", opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("group"), consumer}},
		{name: "canceled context", ctx: canceled, opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("group"), consumer}, wantIs: context.Canceled},
		{name: "missing consumer function", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("group")}},
		{name: "missing topic", ctx: context.Background(), opts: []storage.Option{storage.WithGroupID("group"), consumer}},
		{name: "missing group", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), consumer}},
		{name: "invalid partition", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("group"), storage.WithPartition(256), consumer}},
		{name: "automatic commit disabled", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("group"), storage.WithKafkaConfig(disabledAutoCommit), consumer}},
		{name: "consumer factory error", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("factory-error"), consumer}, wantIs: consumerErr},
		{name: "nil consumer", ctx: context.Background(), opts: []storage.Option{storage.WithTopic("events"), storage.WithGroupID("factory-nil"), consumer}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := queue.RegisterContext(test.ctx, test.opts...)
			if err == nil {
				t.Fatal("RegisterContext() error = nil, want validation/construction error")
			}
			if test.wantIs != nil && !errors.Is(err, test.wantIs) {
				t.Fatalf("RegisterContext() error = %v, want wrapped %v", err, test.wantIs)
			}
		})
	}
	if consumerFactoryCalls.Load() != 2 {
		t.Fatalf("consumer factory calls = %d, want only the 2 construction attempts", consumerFactoryCalls.Load())
	}
}

func TestKafkaLegacyRegisterReportsError(t *testing.T) {
	queue := newKafkaTestQueue(t, nil, nil)
	queue.Register(storage.WithTopic("events"), storage.WithGroupID("group"))
	select {
	case err := <-queue.Errors():
		if err == nil {
			t.Fatal("Errors() yielded nil, want Register validation error")
		}
	case <-time.After(time.Second):
		t.Fatal("legacy Register() did not expose its validation error")
	}
}

func TestKafkaUsesOneProducerForConcurrentAppend(t *testing.T) {
	producer := &kafkaTestProducer{}
	var producerFactoryCalls atomic.Int32
	queue, err := newKafka(
		[]string{"broker.test:9092"},
		sarama.NewConfig(),
		&MessageHandler{},
		"test",
		func([]string, *sarama.Config) (sarama.SyncProducer, error) {
			producerFactoryCalls.Add(1)
			return producer, nil
		},
		func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
			return &kafkaTestConsumerGroup{}, nil
		},
	)
	if err != nil {
		t.Fatalf("newKafka() error = %v", err)
	}

	const appendCount = 32
	start := make(chan struct{})
	results := make(chan error, appendCount)
	var appends sync.WaitGroup
	for index := 0; index < appendCount; index++ {
		index := index
		appends.Add(1)
		go func() {
			defer appends.Done()
			<-start
			results <- queue.Append(storage.WithMessage(kafkaTestMessage(string(rune('a'+index)), "events")))
		}()
	}
	close(start)
	appends.Wait()
	close(results)
	for appendErr := range results {
		if appendErr != nil {
			t.Fatalf("Append() error = %v", appendErr)
		}
	}
	if producerFactoryCalls.Load() != 1 {
		t.Fatalf("producer factory calls = %d, want 1", producerFactoryCalls.Load())
	}
	if producer.sendCalls.Load() != appendCount || len(producer.recordedMessages()) != appendCount {
		t.Fatalf("producer sends = %d/%d, want %d", producer.sendCalls.Load(), len(producer.recordedMessages()), appendCount)
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if producer.closeCalls.Load() != 1 {
		t.Fatalf("producer Close calls = %d, want 1", producer.closeCalls.Load())
	}
}

func TestKafkaAppendRejectsInvalidOptions(t *testing.T) {
	producer := &kafkaTestProducer{}
	queue := newKafkaTestQueue(t, producer, nil)

	tests := []struct {
		name string
		opts []storage.Option
	}{
		{name: "nil message"},
		{name: "empty stream", opts: []storage.Option{storage.WithMessage(kafkaTestMessage("event", ""))}},
		{name: "per-message config", opts: []storage.Option{storage.WithMessage(kafkaTestMessage("event", "events")), storage.WithKafkaConfig(sarama.NewConfig())}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := queue.Append(test.opts...); err == nil {
				t.Fatal("Append() error = nil, want validation error")
			}
		})
	}
	if producer.sendCalls.Load() != 0 {
		t.Fatalf("producer sends = %d, want 0", producer.sendCalls.Load())
	}
}

func TestKafkaDuplicateRegistrationDoesNotConstructSecondGroup(t *testing.T) {
	group := &kafkaTestConsumerGroup{}
	var consumerFactoryCalls atomic.Int32
	queue := newKafkaTestQueue(t, nil, func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		consumerFactoryCalls.Add(1)
		return group, nil
	})
	registerKafkaTestConsumer(t, queue, "events", "workers")
	err := queue.RegisterContext(
		context.Background(),
		storage.WithTopic("events"),
		storage.WithGroupID("workers"),
		storage.WithConsumerFunc(func(storage.Messager) error { return nil }),
	)
	if !errors.Is(err, ErrKafkaDuplicateConsumer) {
		t.Fatalf("duplicate RegisterContext() error = %v, want ErrKafkaDuplicateConsumer", err)
	}
	if consumerFactoryCalls.Load() != 1 {
		t.Fatalf("consumer factory calls = %d, want 1", consumerFactoryCalls.Load())
	}
}

func TestKafkaRegistrationRejectedAfterStart(t *testing.T) {
	group := &kafkaTestConsumerGroup{started: make(chan struct{})}
	var consumerFactoryCalls atomic.Int32
	queue := newKafkaTestQueue(t, nil, func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		consumerFactoryCalls.Add(1)
		return group, nil
	})
	registerKafkaTestConsumer(t, queue, "events", "workers")
	ctx, cancel := context.WithCancel(context.Background())
	startResult := make(chan error, 1)
	go func() { startResult <- queue.Start(ctx) }()
	waitKafkaTestSignal(t, group.started, "Kafka consumer start")

	err := queue.RegisterContext(
		context.Background(),
		storage.WithTopic("other-events"),
		storage.WithGroupID("other-workers"),
		storage.WithConsumerFunc(func(storage.Messager) error { return nil }),
	)
	if !errors.Is(err, ErrKafkaRegistrationAfter) {
		t.Fatalf("RegisterContext() after Start error = %v, want ErrKafkaRegistrationAfter", err)
	}
	if consumerFactoryCalls.Load() != 1 {
		t.Fatalf("consumer factory calls = %d, want 1", consumerFactoryCalls.Load())
	}
	cancel()
	if err := waitKafkaTestError(t, startResult, "Kafka Start cancellation"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestKafkaConsumerErrorIsObservedWithoutSpin(t *testing.T) {
	consumerErrors := make(chan error, 1)
	group := &kafkaTestConsumerGroup{
		started: make(chan struct{}),
		errors:  consumerErrors,
	}
	queue := newKafkaTestQueue(t, nil, func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		return group, nil
	})
	registerKafkaTestConsumer(t, queue, "events", "workers")
	ctx, cancel := context.WithCancel(context.Background())
	startResult := make(chan error, 1)
	go func() { startResult <- queue.Start(ctx) }()
	waitKafkaTestSignal(t, group.started, "Kafka consumer start")

	consumerErr := errors.New("broker connection lost")
	consumerErrors <- consumerErr
	close(consumerErrors)
	select {
	case observed := <-queue.Errors():
		if !errors.Is(observed, consumerErr) {
			t.Fatalf("Errors() = %v, want wrapped consumer error", observed)
		}
	case <-time.After(time.Second):
		t.Fatal("consumer error was not observed")
	}
	cancel()
	if err := waitKafkaTestError(t, startResult, "Kafka Start cancellation"); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if group.errorsCalls.Load() != 1 {
		t.Fatalf("Errors calls = %d, want 1 without a closed-channel spin", group.errorsCalls.Load())
	}
	if group.consumeCalls.Load() != 1 {
		t.Fatalf("Consume calls = %d, want 1", group.consumeCalls.Load())
	}
}

func TestKafkaCloseTimeoutCanBeRetriedAndDrainsOperations(t *testing.T) {
	releaseSend := make(chan struct{})
	producer := &kafkaTestProducer{
		started:     make(chan struct{}),
		sendRelease: releaseSend,
	}
	queue := newKafkaTestQueue(t, producer, nil)
	appendResult := make(chan error, 1)
	go func() {
		appendResult <- queue.Append(storage.WithMessage(kafkaTestMessage("event", "events")))
	}()
	waitKafkaTestSignal(t, producer.started, "in-flight Kafka append")

	firstCloseCtx, cancelFirstClose := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelFirstClose()
	if err := queue.Close(firstCloseCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first Close() error = %v, want context.DeadlineExceeded", err)
	}
	if err := queue.Append(storage.WithMessage(kafkaTestMessage("late", "events"))); !errors.Is(err, ErrKafkaClosed) {
		t.Fatalf("Append() after Close began error = %v, want ErrKafkaClosed", err)
	}

	secondCloseCtx, cancelSecondClose := context.WithTimeout(context.Background(), time.Second)
	defer cancelSecondClose()
	secondCloseResult := make(chan error, 1)
	go func() { secondCloseResult <- queue.Close(secondCloseCtx) }()
	select {
	case err := <-secondCloseResult:
		t.Fatalf("retry Close() returned before the in-flight append drained: %v", err)
	default:
	}
	close(releaseSend)
	if err := waitKafkaTestError(t, appendResult, "in-flight Kafka append"); err != nil {
		t.Fatalf("in-flight Append() error = %v", err)
	}
	if err := waitKafkaTestError(t, secondCloseResult, "retry Kafka Close"); err != nil {
		t.Fatalf("retry Close() error = %v", err)
	}
	if err := queue.Close(context.Background()); err != nil {
		t.Fatalf("idempotent Close() error = %v", err)
	}
	if producer.sendCalls.Load() != 1 {
		t.Fatalf("producer sends = %d, want 1", producer.sendCalls.Load())
	}
	if producer.closeCalls.Load() != 1 {
		t.Fatalf("producer Close calls = %d, want 1", producer.closeCalls.Load())
	}
}

func TestKafkaRunStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	group := &kafkaTestConsumerGroup{started: make(chan struct{})}
	producer := &kafkaTestProducer{}
	kafka := newKafkaTestQueue(t, producer, func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		return group, nil
	})
	registerKafkaTestConsumer(t, kafka, "events", "workers")
	done := make(chan struct{})
	go func() {
		kafka.Run(ctx)
		close(done)
	}()

	waitKafkaTestSignal(t, group.started, "Kafka consumer loop start")
	cancel()
	waitKafkaTestSignal(t, done, "Kafka Run cancellation")
	if calls := group.consumeCalls.Load(); calls != 1 {
		t.Fatalf("Consume calls = %d, want 1 without cancellation spin", calls)
	}
	if calls := group.closeCalls.Load(); calls != 1 {
		t.Fatalf("consumer Close calls = %d, want 1", calls)
	}
	if calls := producer.closeCalls.Load(); calls != 1 {
		t.Fatalf("producer Close calls = %d, want 1", calls)
	}
}

func TestKafkaRunStopsAfterConsumerClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed := make(chan struct{})
	group := &kafkaTestConsumerGroup{
		started: make(chan struct{}),
		closed:  closed,
		consume: func(_ context.Context, call int32) error {
			if call == 1 {
				<-closed
				return nil
			}
			return sarama.ErrClosedConsumerGroup
		},
	}
	producer := &kafkaTestProducer{}
	kafka := newKafkaTestQueue(t, producer, func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error) {
		return group, nil
	})
	registerKafkaTestConsumer(t, kafka, "events", "workers")
	done := make(chan struct{})
	go func() {
		kafka.Run(ctx)
		close(done)
	}()

	waitKafkaTestSignal(t, group.started, "Kafka consumer loop start")
	shutdownDone := make(chan struct{})
	go func() {
		kafka.Shutdown()
		close(shutdownDone)
	}()
	waitKafkaTestSignal(t, shutdownDone, "Kafka Shutdown")
	waitKafkaTestSignal(t, done, "Kafka Run after Shutdown")
	if calls := group.consumeCalls.Load(); calls != 1 {
		t.Fatalf("Consume calls = %d, want 1 without a post-close spin", calls)
	}
	if calls := group.closeCalls.Load(); calls != 1 {
		t.Fatalf("Close calls = %d, want 1", calls)
	}
	if calls := producer.closeCalls.Load(); calls != 1 {
		t.Fatalf("producer Close calls = %d, want 1", calls)
	}
}
