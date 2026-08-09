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
	closeCalls   atomic.Int32
	started      chan struct{}
	startOnce    sync.Once
	closeOnce    sync.Once
	closed       chan struct{}
	consume      func(context.Context, int32) error
}

func (g *kafkaTestConsumerGroup) Consume(ctx context.Context, _ []string, _ sarama.ConsumerGroupHandler) error {
	calls := g.consumeCalls.Add(1)
	g.startOnce.Do(func() { close(g.started) })
	if g.consume != nil {
		return g.consume(ctx, calls)
	}
	<-ctx.Done()
	return ctx.Err()
}

func (*kafkaTestConsumerGroup) Errors() <-chan error      { return nil }
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

func TestKafkaRunStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	group := &kafkaTestConsumerGroup{started: make(chan struct{})}
	kafka := &Kafka{}
	register := &ConsumerRegister{Topic: "events", Func: &MessageHandler{}}
	done := make(chan struct{})
	go func() {
		kafka.runConsumer(ctx, register, group)
		close(done)
	}()

	select {
	case <-group.started:
	case <-time.After(time.Second):
		t.Fatal("consumer loop did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer loop did not exit after cancellation")
	}
	if calls := group.consumeCalls.Load(); calls != 1 {
		t.Fatalf("Consume calls = %d, want 1 without cancellation spin", calls)
	}
}

func TestKafkaRunStopsAfterConsumerClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
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
	register := &ConsumerRegister{Topic: "events", Func: &MessageHandler{}}
	kafka := &Kafka{consumers: map[*ConsumerRegister]sarama.ConsumerGroup{register: group}}
	done := make(chan struct{})
	go func() {
		kafka.runConsumer(ctx, register, group)
		close(done)
	}()

	select {
	case <-group.started:
	case <-time.After(time.Second):
		t.Fatal("consumer loop did not start")
	}
	kafka.Shutdown()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("consumer loop did not exit after Shutdown closed the consumer group")
	}
	if calls := group.consumeCalls.Load(); calls != 2 {
		t.Fatalf("Consume calls = %d, want the active call and one ErrClosedConsumerGroup observation", calls)
	}
	if calls := group.closeCalls.Load(); calls != 1 {
		t.Fatalf("Close calls = %d, want 1", calls)
	}
}
