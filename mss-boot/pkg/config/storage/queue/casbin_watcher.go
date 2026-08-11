package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/config/storage"
)

func NewSampleWatcher(queue storage.AdapterQueue) *SampleWatcher {
	return &SampleWatcher{
		queue:   queue,
		topic:   "casbin-watcher",
		groupID: pkg.GetNodeName(),
	}
}

type SampleWatcher struct {
	queue      storage.AdapterQueue
	topic      string
	groupID    string
	callbackMu sync.RWMutex
	callback   func(string)
	registered bool
}

func (w *SampleWatcher) Close() {
}

func (w *SampleWatcher) SetUpdateCallback(callback func(string)) error {
	if _, managed := w.queue.(storage.ManagedAdapterQueue); managed {
		w.callbackMu.Lock()
		defer w.callbackMu.Unlock()
		if !w.registered {
			return errors.New("managed watcher registration requires SetUpdateCallbackContext")
		}
		// Casbin calls SetUpdateCallback again from Enforcer.SetWatcher. Once
		// RegisterContext has succeeded, that call updates only the callback and
		// must not create a duplicate topic/group consumer.
		w.callback = callback
		return nil
	}
	return w.setUpdateCallback(nil, callback)
}

// SetUpdateCallbackContext registers the watcher using the caller-owned
// lifecycle when the configured queue exposes the managed queue contract.
// The legacy AdapterQueue path remains available for non-managed providers.
func (w *SampleWatcher) SetUpdateCallbackContext(ctx context.Context, callback func(string)) error {
	if ctx == nil {
		return errors.New("watcher registration context is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("watcher registration context: %w", err)
	}
	return w.setUpdateCallback(ctx, callback)
}

func (w *SampleWatcher) setUpdateCallback(ctx context.Context, callback func(string)) error {
	if w.queue == nil {
		return errors.New("queue is nil")
	}
	options := []storage.Option{
		storage.WithTopic(fmt.Sprintf("%s-%s", w.topic, pkg.GetStage())),
		storage.WithGroupID(w.groupID),
		storage.WithConsumerFunc(func(message storage.Messager) error {
			if message.GetID() == w.groupID {
				return nil
			}
			w.callbackMu.RLock()
			current := w.callback
			w.callbackMu.RUnlock()
			if current != nil {
				current(message.GetID())
			}
			return nil
		}),
	}
	if managed, ok := w.queue.(storage.ManagedAdapterQueue); ok {
		if ctx == nil {
			return errors.New("managed watcher registration requires SetUpdateCallbackContext")
		}
		if err := managed.RegisterContext(ctx, options...); err != nil {
			return fmt.Errorf("register managed Casbin watcher: %w", err)
		}
		w.setManagedCallback(callback)
		return nil
	}
	w.setCallback(callback)
	w.queue.Register(options...)
	return nil
}

func (w *SampleWatcher) setCallback(callback func(string)) {
	w.callbackMu.Lock()
	w.callback = callback
	w.callbackMu.Unlock()
}

func (w *SampleWatcher) setManagedCallback(callback func(string)) {
	w.callbackMu.Lock()
	w.callback = callback
	w.registered = true
	w.callbackMu.Unlock()
}

func (w *SampleWatcher) Update() error {
	if w.queue == nil {
		return errors.New("queue is nil")
	}
	message := &Message{}
	message.SetStream(fmt.Sprintf("%s-%s", w.topic, pkg.GetStage()))
	message.SetID(w.groupID)
	message.SetValues(map[string]interface{}{
		"self": w.groupID,
	})
	return w.queue.Append(
		storage.WithTopic(fmt.Sprintf("%s-%s", w.topic, pkg.GetStage())),
		storage.WithMessage(message),
	)
}
