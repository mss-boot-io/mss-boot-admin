package response

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	chTranslations "github.com/go-playground/validator/v10/translations/zh"
)

var validationTranslations struct {
	once sync.Once
	en   ut.Translator
	zh   ut.Translator
	err  error
}

// transInit returns a pre-registered validator translator. Registration mutates
// validator state, so it is performed exactly once before concurrent requests
// begin translating validation errors.
func transInit(locale string) (ut.Translator, error) {
	validationTranslations.once.Do(func() {
		engine := binding.Validator.Engine()
		validate, ok := engine.(*validator.Validate)
		if !ok || validate == nil {
			validationTranslations.err = fmt.Errorf("unsupported validator engine %T", engine)
			return
		}

		zhLocale := zh.New()
		enLocale := en.New()
		universal := ut.New(enLocale, zhLocale, enLocale)

		var okTranslator bool
		validationTranslations.zh, okTranslator = universal.GetTranslator("zh")
		if !okTranslator {
			validationTranslations.err = errors.New("validation translator zh is unavailable")
			return
		}
		validationTranslations.en, okTranslator = universal.GetTranslator("en")
		if !okTranslator {
			validationTranslations.err = errors.New("validation translator en is unavailable")
			return
		}

		validationTranslations.err = errors.Join(
			chTranslations.RegisterDefaultTranslations(validate, validationTranslations.zh),
			enTranslations.RegisterDefaultTranslations(validate, validationTranslations.en),
		)
	})
	if validationTranslations.err != nil {
		return nil, validationTranslations.err
	}

	switch locale {
	case "zh", "zh-CN", "zh-Hans":
		return validationTranslations.zh, nil
	default:
		return validationTranslations.en, nil
	}
}
