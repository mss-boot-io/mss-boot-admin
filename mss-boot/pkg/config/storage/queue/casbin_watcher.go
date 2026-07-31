package queue

import (
	"errors"
	"fmt"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
)

func NewSampleWatcher(queue storage.AdapterQueue) *SampleWatcher {
	return &SampleWatcher{
		queue:   queue,
		topic:   "casbin-watcher",
		groupID: pkg.GetNodeName(),
	}
}

type SampleWatcher struct {
	queue    storage.AdapterQueue
	topic    string
	groupID  string
	callback func(string)
}

func (w *SampleWatcher) Close() {
}

func (w *SampleWatcher) SetUpdateCallback(callback func(string)) error {
	w.callback = callback
	if w.queue == nil {
		return errors.New("queue is nil")
	}
	w.queue.Register(
		storage.WithTopic(fmt.Sprintf("%s-%s", w.topic, pkg.GetStage())),
		storage.WithGroupID(w.groupID),
		storage.WithConsumerFunc(func(message storage.Messager) error {
			if message.GetID() == w.groupID {
				return nil
			}
			if w.callback != nil {
				w.callback(message.GetID())
			}
			return nil
		}),
	)
	return nil
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
