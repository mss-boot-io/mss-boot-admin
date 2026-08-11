package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

const (
	kafkaLegacyRegistrationTimeout = 30 * time.Second
	kafkaLegacyShutdownTimeout     = 10 * time.Second
)

var (
	ErrKafkaClosed            = errors.New("Kafka queue is closed")
	ErrKafkaAlreadyRunning    = errors.New("Kafka queue is already running")
	ErrKafkaRegistrationAfter = errors.New("Kafka consumer registration is closed after Start")
	ErrKafkaConsumerPending   = errors.New("Kafka consumer registration is already pending")
	ErrKafkaRegistrationsOpen = errors.New("Kafka consumer registrations are still pending")
	ErrKafkaDuplicateConsumer = errors.New("Kafka consumer registration already exists")
)

type ConsumerGroupHandler interface {
	sarama.ConsumerGroupHandler
	SetConsumerFunc(f storage.ConsumerFunc)
}

type kafkaProducerFactory func([]string, *sarama.Config) (sarama.SyncProducer, error)
type kafkaConsumerFactory func([]string, string, *sarama.Config) (sarama.ConsumerGroup, error)

func NewKafka(
	brokers []string,
	configuration *sarama.Config,
	handler ConsumerGroupHandler,
	provider string,
) (*Kafka, error) {
	return newKafka(
		brokers,
		configuration,
		handler,
		provider,
		sarama.NewSyncProducer,
		sarama.NewConsumerGroup,
	)
}

func newKafka(
	brokers []string,
	configuration *sarama.Config,
	handler ConsumerGroupHandler,
	provider string,
	producerFactory kafkaProducerFactory,
	consumerFactory kafkaConsumerFactory,
) (*Kafka, error) {
	cleanBrokers, err := validateKafkaBrokers(brokers)
	if err != nil {
		return nil, invalidKafkaConfiguration(err)
	}
	if configuration == nil {
		return nil, invalidKafkaConfiguration(errors.New("configuration is required"))
	}
	cleanProvider := strings.ToLower(strings.TrimSpace(provider))
	if cleanProvider != "" && cleanProvider != "kafka" && cleanProvider != "msk" {
		return nil, invalidKafkaConfiguration(fmt.Errorf("unsupported provider %q", provider))
	}
	if producerFactory == nil || consumerFactory == nil {
		return nil, errors.New("Kafka factories are required")
	}
	if _, err := cloneConsumerGroupHandler(handler); err != nil {
		return nil, invalidKafkaConfiguration(err)
	}

	ownedConfig := cloneKafkaConfig(configuration)
	ownedConfig.Producer.Return.Errors = true
	ownedConfig.Producer.Return.Successes = true
	ownedConfig.Consumer.Return.Errors = true
	if !ownedConfig.Consumer.Offsets.AutoCommit.Enable {
		return nil, invalidKafkaConfiguration(errors.New("automatic offset commit is required for MarkMessage delivery"))
	}
	if err := ownedConfig.Validate(); err != nil {
		return nil, invalidKafkaConfiguration(fmt.Errorf("validate configuration: %w", err))
	}
	producer, err := producerFactory(cleanBrokers, &ownedConfig)
	if err != nil {
		return nil, classifyKafkaFactoryError("create owned producer", err)
	}
	if producer == nil {
		return nil, errors.New("create owned Kafka producer: producer is nil")
	}

	return &Kafka{
		brokers:              cleanBrokers,
		config:               ownedConfig,
		producer:             producer,
		consumerGroupHandler: handler,
		provider:             cleanProvider,
		consumers:            make(map[kafkaConsumerKey]*kafkaConsumer),
		pendingConsumers:     make(map[kafkaConsumerKey]*kafkaPendingConsumer),
		consumerFactory:      consumerFactory,
		errorCh:              make(chan error, 32),
	}, nil
}

func invalidKafkaConfiguration(err error) error {
	return &storage.InvalidConfigurationError{Adapter: "Kafka", Err: err}
}

func classifyKafkaFactoryError(operation string, err error) error {
	wrapped := fmt.Errorf("%s: %w", operation, err)
	if kafkaDependencyIsUnavailable(err) {
		return &storage.DependencyUnavailableError{Adapter: "Kafka", Err: wrapped}
	}
	return wrapped
}

func kafkaDependencyIsUnavailable(err error) bool {
	if errors.Is(err, sarama.ErrOutOfBrokers) ||
		errors.Is(err, sarama.ErrRequestTimedOut) ||
		errors.Is(err, sarama.ErrBrokerNotAvailable) ||
		errors.Is(err, sarama.ErrNetworkException) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError)
}

func validateKafkaBrokers(brokers []string) ([]string, error) {
	if len(brokers) == 0 {
		return nil, errors.New("at least one Kafka broker is required")
	}
	clean := make([]string, 0, len(brokers))
	seen := make(map[string]struct{}, len(brokers))
	for index, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			return nil, fmt.Errorf("Kafka broker %d is empty", index)
		}
		host, portText, splitErr := net.SplitHostPort(broker)
		if splitErr != nil || strings.TrimSpace(host) == "" {
			return nil, fmt.Errorf("Kafka broker %d must use host:port syntax", index)
		}
		port, portErr := strconv.Atoi(portText)
		if portErr != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("Kafka broker %d has invalid port %q", index, portText)
		}
		if _, exists := seen[broker]; exists {
			continue
		}
		seen[broker] = struct{}{}
		clean = append(clean, broker)
	}
	return clean, nil
}

func cloneKafkaConfig(configuration *sarama.Config) sarama.Config {
	cloned := *configuration
	cloned.Consumer.Group.Rebalance.GroupStrategies = append(
		[]sarama.BalanceStrategy(nil),
		configuration.Consumer.Group.Rebalance.GroupStrategies...,
	)
	cloned.Consumer.Group.Member.UserData = append(
		[]byte(nil),
		configuration.Consumer.Group.Member.UserData...,
	)
	if configuration.Net.TLS.Config != nil {
		cloned.Net.TLS.Config = configuration.Net.TLS.Config.Clone()
	}
	return cloned
}

type ConsumerRegister struct {
	Topic     string
	GroupID   string
	Partition int
	Func      ConsumerGroupHandler
}

// KafkaRunReader is retained as a deprecated source-compatibility bridge for
// integrations that referenced the v1.0 exported registration descriptor.
// Use ManagedAdapterQueue.RegisterContext with storage options instead.
//
// Deprecated: use ManagedAdapterQueue.RegisterContext.
type KafkaRunReader struct {
	Topic   string
	GroupID string
	Func    storage.ConsumerFunc
}

type kafkaConsumerKey struct {
	topic   string
	groupID string
}

type kafkaConsumer struct {
	register ConsumerRegister
	group    sarama.ConsumerGroup
}

type kafkaPendingConsumer struct {
	done                 chan struct{}
	abandoned            error
	completed            bool
	resultErr            error
	beforeAcceptComplete func()
}

type Kafka struct {
	mu sync.Mutex

	consumers             map[kafkaConsumerKey]*kafkaConsumer
	pendingConsumers      map[kafkaConsumerKey]*kafkaPendingConsumer
	brokers               []string
	config                sarama.Config
	producer              sarama.SyncProducer
	consumerGroupHandler  ConsumerGroupHandler
	provider              string
	consumerFactory       kafkaConsumerFactory
	legacyRegisterTimeout time.Duration

	operations           sync.WaitGroup
	registrations        sync.WaitGroup
	runners              sync.WaitGroup
	runCancel            context.CancelFunc
	running              bool
	closing              bool
	closeDone            chan struct{}
	closeErr             error
	registrationCloseErr error
	lastErr              error
	errorCh              chan error
	errorsDone           bool
}

func (*Kafka) String() string {
	return "kafka"
}

func (e *Kafka) Errors() <-chan error {
	if e == nil {
		return nil
	}
	return e.errorCh
}

func (e *Kafka) Err() error {
	if e == nil {
		return ErrKafkaClosed
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastErr
}

func (e *Kafka) Append(opts ...storage.Option) error {
	if e == nil {
		return ErrKafkaClosed
	}
	o := storage.SetOptions(opts...)
	if o.Message == nil {
		return errors.New("Kafka message is required")
	}
	if o.KafkaConfig != nil {
		return errors.New("per-message Kafka configuration is not supported; use the immutable startup profile")
	}
	stream := strings.TrimSpace(o.Message.GetStream())
	if stream == "" {
		return errors.New("Kafka message stream is required")
	}

	producer, err := e.beginOperation()
	if err != nil {
		return err
	}
	defer e.operations.Done()

	rb, err := json.Marshal(o.Message.GetValues())
	if err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: stream,
		Key:   sarama.StringEncoder(o.Message.GetID()),
		Value: sarama.ByteEncoder(rb),
	}
	_, _, err = producer.SendMessage(msg)
	if err != nil {
		err = fmt.Errorf("send Kafka message: %w", err)
		e.reportError(err)
	}
	return err
}

func (e *Kafka) beginOperation() (sarama.SyncProducer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closing || e.producer == nil {
		return nil, ErrKafkaClosed
	}
	e.operations.Add(1)
	return e.producer, nil
}

// Register preserves the legacy AdapterQueue surface without terminating the
// process or waiting forever. New composition roots must call RegisterContext
// and supply their own authoritative context.
func (e *Kafka) Register(opts ...storage.Option) {
	timeout := kafkaLegacyRegistrationTimeout
	if e != nil && e.legacyRegisterTimeout > 0 {
		timeout = e.legacyRegisterTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := e.RegisterContext(ctx, opts...); err != nil {
		e.reportError(err)
	}
}

func (e *Kafka) RegisterContext(ctx context.Context, opts ...storage.Option) error {
	if ctx == nil {
		return errors.New("Kafka registration context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return e.register(ctx, opts...)
}

func (e *Kafka) register(ctx context.Context, opts ...storage.Option) error {
	if e == nil {
		return ErrKafkaClosed
	}
	o := storage.SetOptions(opts...)
	if o.F == nil {
		return invalidKafkaConfiguration(errors.New("consumer function is required"))
	}
	topic := strings.TrimSpace(o.Topic)
	if topic == "" {
		return invalidKafkaConfiguration(errors.New("consumer topic is required"))
	}
	groupID := strings.TrimSpace(o.GroupID)
	if groupID == "" {
		return invalidKafkaConfiguration(errors.New("consumer group ID is required"))
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}

	consumerConfig := cloneKafkaConfig(&e.config)
	if o.KafkaConfig != nil {
		override := cloneKafkaConfig(o.KafkaConfig)
		consumerConfig.Consumer = override.Consumer
	}
	consumerConfig.Consumer.Return.Errors = true
	if !consumerConfig.Consumer.Offsets.AutoCommit.Enable {
		return invalidKafkaConfiguration(errors.New("automatic offset commit is required for MarkMessage delivery"))
	}
	if o.PartitionAssignmentStrategy != nil {
		consumerConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{o.PartitionAssignmentStrategy}
	}
	if o.Partition >= 0 {
		if o.Partition > 255 {
			return invalidKafkaConfiguration(errors.New("partition hint must be between 0 and 255"))
		}
		consumerConfig.Consumer.Group.Member.UserData = []byte{byte(o.Partition)}
	}
	if err := consumerConfig.Validate(); err != nil {
		return invalidKafkaConfiguration(fmt.Errorf("validate consumer configuration: %w", err))
	}
	handler, err := cloneConsumerGroupHandler(e.consumerGroupHandler)
	if err != nil {
		return invalidKafkaConfiguration(err)
	}
	handler.SetConsumerFunc(o.F)

	key := kafkaConsumerKey{topic: topic, groupID: groupID}
	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		return ErrKafkaClosed
	}
	if e.running {
		e.mu.Unlock()
		return ErrKafkaRegistrationAfter
	}
	if e.consumers[key] != nil {
		e.mu.Unlock()
		return fmt.Errorf("%w: topic %q group %q", ErrKafkaDuplicateConsumer, topic, groupID)
	}
	if _, pending := e.pendingConsumers[key]; pending {
		e.mu.Unlock()
		return fmt.Errorf("%w: topic %q group %q", ErrKafkaConsumerPending, topic, groupID)
	}
	if e.pendingConsumers == nil {
		e.pendingConsumers = make(map[kafkaConsumerKey]*kafkaPendingConsumer)
	}
	pending := &kafkaPendingConsumer{done: make(chan struct{})}
	e.pendingConsumers[key] = pending
	e.registrations.Go(func() {
		e.constructConsumerGroup(
			key,
			pending,
			ConsumerRegister{Topic: topic, GroupID: groupID, Partition: o.Partition, Func: handler},
			&consumerConfig,
		)
	})
	e.mu.Unlock()

	if ctx == nil {
		<-pending.done
		e.mu.Lock()
		err := pending.resultErr
		e.mu.Unlock()
		return err
	}
	select {
	case <-pending.done:
		e.mu.Lock()
		err := pending.resultErr
		e.mu.Unlock()
		return err
	case <-ctx.Done():
		e.mu.Lock()
		if pending.completed {
			err := pending.resultErr
			e.mu.Unlock()
			return err
		}
		if pending.abandoned == nil {
			pending.abandoned = ctx.Err()
		}
		e.mu.Unlock()
		return ctx.Err()
	}
}

func (e *Kafka) constructConsumerGroup(
	key kafkaConsumerKey,
	pending *kafkaPendingConsumer,
	register ConsumerRegister,
	configuration *sarama.Config,
) {
	// Sarama construction may dial brokers. It must never run while mu is held:
	// Close remains deadline-bounded, callers can abandon on their own context,
	// and duplicate callers observe one tracked pending reservation.
	group, factoryErr := e.consumerFactory(e.brokers, register.GroupID, configuration)
	if factoryErr != nil {
		e.completePendingRegistration(
			key,
			pending,
			classifyKafkaFactoryError(fmt.Sprintf("create consumer group %q", register.GroupID), factoryErr),
		)
		return
	}
	if group == nil {
		e.completePendingRegistration(
			key,
			pending,
			fmt.Errorf("create Kafka consumer group %q: group is nil", register.GroupID),
		)
		return
	}

	var rejectErr error
	e.mu.Lock()
	delete(e.pendingConsumers, key)
	if pending.abandoned != nil {
		rejectErr = pending.abandoned
	}
	if rejectErr == nil && e.closing {
		rejectErr = ErrKafkaClosed
	}
	if rejectErr == nil && e.running {
		rejectErr = ErrKafkaRegistrationAfter
	}
	if rejectErr == nil {
		e.consumers[key] = &kafkaConsumer{
			register: register,
			group:    group,
		}
		if pending.beforeAcceptComplete != nil {
			pending.beforeAcceptComplete()
		}
		// Acceptance and completion are one linearization point. A caller
		// whose context becomes ready after the group is accepted must observe
		// the completed success rather than return cancellation while leaving
		// an accepted consumer behind.
		e.finishPendingResultLocked(pending, nil)
	}
	e.mu.Unlock()
	if rejectErr == nil {
		return
	}

	closeErr := group.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf(
			"close rejected Kafka consumer topic %q group %q: %w",
			register.Topic,
			register.GroupID,
			closeErr,
		)
		e.mu.Lock()
		e.registrationCloseErr = errors.Join(e.registrationCloseErr, closeErr)
		e.mu.Unlock()
	}
	e.finishPendingResult(pending, errors.Join(rejectErr, closeErr))
}

func (e *Kafka) completePendingRegistration(
	key kafkaConsumerKey,
	pending *kafkaPendingConsumer,
	err error,
) {
	e.mu.Lock()
	delete(e.pendingConsumers, key)
	pending.resultErr = err
	pending.completed = true
	close(pending.done)
	e.mu.Unlock()
}

func (e *Kafka) finishPendingResult(pending *kafkaPendingConsumer, err error) {
	e.mu.Lock()
	e.finishPendingResultLocked(pending, err)
	e.mu.Unlock()
}

func (e *Kafka) finishPendingResultLocked(pending *kafkaPendingConsumer, err error) {
	pending.resultErr = err
	pending.completed = true
	close(pending.done)
}

func cloneConsumerGroupHandler(prototype ConsumerGroupHandler) (ConsumerGroupHandler, error) {
	if prototype == nil {
		return nil, errors.New("Kafka consumer group handler prototype is required")
	}
	typeOf := reflect.TypeOf(prototype)
	if typeOf.Kind() != reflect.Pointer || typeOf.Elem().Kind() != reflect.Struct {
		return nil, errors.New("Kafka consumer group handler prototype must be a pointer to a struct")
	}
	cloned, ok := reflect.New(typeOf.Elem()).Interface().(ConsumerGroupHandler)
	if !ok {
		return nil, errors.New("Kafka consumer group handler clone does not implement ConsumerGroupHandler")
	}
	return cloned, nil
}

// Run preserves AdapterQueue. Managed callers should register Kafka as a
// server Runnable and call Start so runtime errors are propagated.
func (e *Kafka) Run(ctx context.Context) {
	if err := e.Start(ctx); err != nil {
		e.reportError(err)
	}
}

// Start blocks until cancellation, a consumer fails, or Close begins. It owns
// every consumer/error-observer goroutine and closes all Kafka clients before
// returning.
func (e *Kafka) Start(ctx context.Context) error {
	if e == nil {
		return ErrKafkaClosed
	}
	if ctx == nil {
		return errors.New("Kafka run context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	e.mu.Lock()
	if e.closing {
		e.mu.Unlock()
		return ErrKafkaClosed
	}
	if e.running {
		e.mu.Unlock()
		return ErrKafkaAlreadyRunning
	}
	if len(e.pendingConsumers) != 0 {
		e.mu.Unlock()
		return ErrKafkaRegistrationsOpen
	}
	e.running = true
	runCtx, cancel := context.WithCancel(ctx)
	e.runCancel = cancel
	consumers := e.sortedConsumersLocked()
	results := make(chan error, len(consumers))
	for range consumers {
		e.runners.Add(2)
	}
	e.mu.Unlock()

	for _, consumer := range consumers {
		consumer := consumer
		go func() {
			defer e.runners.Done()
			results <- e.runConsumer(runCtx, consumer)
		}()
		go func() {
			defer e.runners.Done()
			e.observeConsumerErrors(runCtx, consumer)
		}()
	}

	var runErr error
	if len(consumers) == 0 {
		<-runCtx.Done()
	} else {
		select {
		case <-runCtx.Done():
		case runErr = <-results:
			if runErr != nil {
				e.reportError(runErr)
			}
		}
	}
	cancel()
	e.startClose()
	closeErr := e.waitClose(kafkaLegacyShutdownTimeout)
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, sarama.ErrClosedConsumerGroup) {
		runErr = nil
	}
	return errors.Join(runErr, closeErr)
}

func (e *Kafka) sortedConsumersLocked() []*kafkaConsumer {
	keys := make([]kafkaConsumerKey, 0, len(e.consumers))
	for key := range e.consumers {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].topic == keys[j].topic {
			return keys[i].groupID < keys[j].groupID
		}
		return keys[i].topic < keys[j].topic
	})
	consumers := make([]*kafkaConsumer, 0, len(keys))
	for _, key := range keys {
		consumers = append(consumers, e.consumers[key])
	}
	return consumers
}

func (e *Kafka) runConsumer(ctx context.Context, consumer *kafkaConsumer) error {
	for ctx.Err() == nil {
		err := consumer.group.Consume(ctx, []string{consumer.register.Topic}, consumer.register.Func)
		if ctx.Err() != nil || errors.Is(err, sarama.ErrClosedConsumerGroup) {
			return nil
		}
		if err != nil {
			return fmt.Errorf(
				"consume Kafka topic %q group %q: %w",
				consumer.register.Topic,
				consumer.register.GroupID,
				err,
			)
		}
	}
	return nil
}

func (e *Kafka) observeConsumerErrors(ctx context.Context, consumer *kafkaConsumer) {
	errorsCh := consumer.group.Errors()
	if errorsCh == nil {
		<-ctx.Done()
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errorsCh:
			if !ok {
				return
			}
			if err != nil {
				e.reportError(fmt.Errorf(
					"Kafka consumer topic %q group %q: %w",
					consumer.register.Topic,
					consumer.register.GroupID,
					err,
				))
			}
		}
	}
}

// Shutdown preserves AdapterQueue without Exit/Fatal or an unbounded wait.
// Close(ctx) is the authoritative lifecycle API.
func (e *Kafka) Shutdown() {
	if e == nil {
		return
	}
	e.startClose()
	if err := e.waitClose(kafkaLegacyShutdownTimeout); err != nil {
		e.reportError(err)
	}
}

func (e *Kafka) Close(ctx context.Context) error {
	if e == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("Kafka close context is required")
	}
	e.startClose()
	e.mu.Lock()
	done := e.closeDone
	e.mu.Unlock()
	// A completed close is authoritative even when a retry arrives with an
	// already-canceled context. Preserve terminal producer/consumer diagnostics
	// instead of selecting the caller cancellation nondeterministically.
	select {
	case <-done:
		e.mu.Lock()
		err := e.closeErr
		e.mu.Unlock()
		return err
	default:
	}
	select {
	case <-done:
		e.mu.Lock()
		err := e.closeErr
		e.mu.Unlock()
		return err
	case <-ctx.Done():
		select {
		case <-done:
			e.mu.Lock()
			err := e.closeErr
			e.mu.Unlock()
			return err
		default:
		}
		return ctx.Err()
	}
}

// CloseComplete reports whether all owned producer, consumer, runner,
// operation, and in-flight registration cleanup has reached a terminal state.
func (e *Kafka) CloseComplete() bool {
	if e == nil {
		return true
	}
	e.mu.Lock()
	done := e.closeDone
	e.mu.Unlock()
	if done == nil {
		return false
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}

func (e *Kafka) startClose() {
	e.mu.Lock()
	if e.closeDone != nil {
		e.mu.Unlock()
		return
	}
	e.closing = true
	e.closeDone = make(chan struct{})
	done := e.closeDone
	cancel := e.runCancel
	consumers := e.sortedConsumersLocked()
	producer := e.producer
	e.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	go e.finishClose(done, consumers, producer)
}

func (e *Kafka) finishClose(
	done chan struct{},
	consumers []*kafkaConsumer,
	producer sarama.SyncProducer,
) {
	var closeErr error
	for _, consumer := range consumers {
		if consumer == nil || consumer.group == nil {
			continue
		}
		if err := consumer.group.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf(
				"close Kafka consumer topic %q group %q: %w",
				consumer.register.Topic,
				consumer.register.GroupID,
				err,
			))
		}
	}
	// A registration reserved before closing may still be blocked in Sarama's
	// factory. The caller's Close context remains authoritative while this owned
	// close worker waits; a late group is rejected and closed before Done.
	e.registrations.Wait()
	e.runners.Wait()
	e.operations.Wait()
	if producer != nil {
		if err := producer.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close Kafka producer: %w", err))
		}
	}

	e.mu.Lock()
	closeErr = errors.Join(closeErr, e.registrationCloseErr)
	e.closeErr = closeErr
	e.lastErr = errors.Join(e.lastErr, closeErr)
	e.producer = nil
	e.consumers = nil
	if closeErr != nil && !e.errorsDone {
		select {
		case e.errorCh <- closeErr:
		default:
		}
	}
	if !e.errorsDone {
		close(e.errorCh)
		e.errorsDone = true
	}
	close(done)
	e.mu.Unlock()
}

func (e *Kafka) waitClose(timeout time.Duration) error {
	e.mu.Lock()
	done := e.closeDone
	e.mu.Unlock()
	if done == nil {
		return nil
	}
	if timeout <= 0 {
		<-done
		e.mu.Lock()
		err := e.closeErr
		e.mu.Unlock()
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		e.mu.Lock()
		err := e.closeErr
		e.mu.Unlock()
		return err
	case <-timer.C:
		return fmt.Errorf("Kafka close timed out after %s", timeout)
	}
}

func (e *Kafka) reportError(err error) {
	if e == nil || err == nil || errors.Is(err, context.Canceled) || errors.Is(err, sarama.ErrClosedConsumerGroup) {
		return
	}
	slog.Error("Kafka queue error", "err", err)
	e.mu.Lock()
	e.lastErr = errors.Join(e.lastErr, err)
	if !e.errorsDone {
		select {
		case e.errorCh <- err:
		default:
		}
	}
	e.mu.Unlock()
}

type MessageHandler struct {
	mu sync.RWMutex
	f  storage.ConsumerFunc
}

func (h *MessageHandler) Setup(s sarama.ConsumerGroupSession) error {
	slog.Debug("Kafka partition allocation", slog.Any("claims", s.Claims()))
	return nil
}

func (h *MessageHandler) Cleanup(sarama.ConsumerGroupSession) error {
	slog.Debug("Kafka consumer group cleanup initiated")
	return nil
}

func (h *MessageHandler) ConsumeClaim(s sarama.ConsumerGroupSession, c sarama.ConsumerGroupClaim) error {
	if h.consumerFunc() == nil {
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

			slog.Debug("Kafka message received",
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
			consumer := h.consumerFunc()
			if consumer == nil {
				return errors.New("consumer func is nil")
			}
			if err := consumer(message); err != nil {
				return fmt.Errorf("handle Kafka message: %w", err)
			}
			if s.Context().Err() != nil {
				return nil
			}
			s.MarkMessage(msg, "")
		}
	}
}

func (h *MessageHandler) SetConsumerFunc(f storage.ConsumerFunc) {
	h.mu.Lock()
	h.f = f
	h.mu.Unlock()
}

func (h *MessageHandler) consumerFunc() storage.ConsumerFunc {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.f
}
