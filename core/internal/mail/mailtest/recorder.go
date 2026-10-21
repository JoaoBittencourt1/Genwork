package mailtest

import (
	"context"
	"errors"
	"sync"

	"github.com/JoaoBittencourt1/Genwork/core/internal/mail"
)

var ErrDeliveryFailed = errors.New("delivery failed")

type Recorder struct {
	mutex    sync.Mutex
	messages []mail.Message
	failing  bool
}

func (recorder *Recorder) Send(_ context.Context, message mail.Message) error {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	if recorder.failing {
		return ErrDeliveryFailed
	}
	recorder.messages = append(recorder.messages, message)
	return nil
}

func (recorder *Recorder) FailDeliveries(failing bool) {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	recorder.failing = failing
}

func (recorder *Recorder) Messages() []mail.Message {
	recorder.mutex.Lock()
	defer recorder.mutex.Unlock()
	return append([]mail.Message(nil), recorder.messages...)
}

func (recorder *Recorder) Last() (mail.Message, bool) {
	messages := recorder.Messages()
	if len(messages) == 0 {
		return mail.Message{}, false
	}
	return messages[len(messages)-1], true
}
