package response

import (
	"sync"
	"testing"
)

func TestValidationTranslatorsAreSafeForConcurrentUse(t *testing.T) {
	locales := []string{"zh-CN", "en-US", "zh-Hans", "en"}
	var wait sync.WaitGroup
	errorsCh := make(chan error, 64)
	for i := 0; i < 64; i++ {
		locale := locales[i%len(locales)]
		wait.Add(1)
		go func() {
			defer wait.Done()
			translator, err := transInit(locale)
			if err != nil {
				errorsCh <- err
				return
			}
			if translator == nil {
				errorsCh <- errNilTranslator
			}
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("translator initialization failed: %v", err)
	}
}

type nilTranslatorError struct{}

func (nilTranslatorError) Error() string { return "translator is nil" }

var errNilTranslator error = nilTranslatorError{}
