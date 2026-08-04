package response

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/language"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/security"
)

// DefaultLanguage is used when Accept-Language is absent.
var DefaultLanguage = "zh-CN"

// AuthHandler is the default authentication middleware used by controllers.
var AuthHandler gin.HandlerFunc

// VerifyHandler resolves the request verifier.
var VerifyHandler func(ctx *gin.Context) security.Verifier

// API contains request-scoped response state.
type API struct {
	Context  *gin.Context
	Log      *slog.Logger
	Error    error
	engine   *gin.RouterGroup
	language string
}

// Path returns the route path.
func (*API) Path() string { return "" }

// Handlers returns middleware registered by the API.
func (*API) Handlers() gin.HandlersChain { return nil }

// Create responds with Method Not Allowed by default.
func (*API) Create(c *gin.Context) { methodNotAllowed(c) }

// Update responds with Method Not Allowed by default.
func (*API) Update(c *gin.Context) { methodNotAllowed(c) }

// Delete responds with Method Not Allowed by default.
func (*API) Delete(c *gin.Context) { methodNotAllowed(c) }

// Get responds with Method Not Allowed by default.
func (*API) Get(c *gin.Context) { methodNotAllowed(c) }

// List responds with Method Not Allowed by default.
func (*API) List(c *gin.Context) { methodNotAllowed(c) }

// Other registers additional routes.
func (*API) Other(_ *gin.RouterGroup) {}

// SetEngine sets the router group.
func (e *API) SetEngine(engine *gin.RouterGroup) { e.engine = engine }

// AddError adds an error to the request state without losing prior causes.
func (e *API) AddError(err error) *API {
	if e == nil || err == nil {
		return e
	}
	if e.Error == nil {
		e.Error = err
	} else {
		e.Error = errors.Join(e.Error, err)
	}
	if e.Log == nil {
		e.Log = slog.Default()
	}
	e.Log = e.Log.With("error", e.Error)
	return e
}

// Make sets the HTTP context on an existing API.
func (e *API) Make(c *gin.Context) *API {
	e.Context = c
	e.Log = GetRequestLogger(c)
	return e
}

// Make creates a request-scoped API.
func Make(c *gin.Context) *API {
	return &API{Context: c, Log: GetRequestLogger(c)}
}

// Bind populates d from URI, query/form, and at most one request-body format,
// then validates the fully populated object once.
func (e *API) Bind(d any, bindings ...binding.Binding) *API {
	if e == nil {
		return e
	}
	if e.Context == nil || e.Context.Request == nil {
		return e.AddError(errors.New("response: request context is nil"))
	}
	if d == nil {
		return e.AddError(errors.New("response: bind destination is nil"))
	}
	if len(bindings) == 0 {
		bindings = constructor.GetBindingForGin(d)
	}
	bindings = normalizeBindings(e.Context.Request, bindings)
	if e.Log == nil {
		e.Log = slog.Default()
	}

	for _, requestBinding := range bindings {
		err := e.bindWith(d, requestBinding)
		if errors.Is(err, io.EOF) {
			e.Log.Debug("request body is empty")
			continue
		}
		if err == nil {
			continue
		}
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			// Individual binders validate partial state. Defer validation until URI,
			// query values, and the selected body have all been applied.
			continue
		}
		return e.AddError(err)
	}

	if binding.Validator == nil {
		return e
	}
	if err := binding.Validator.ValidateStruct(d); err != nil {
		return e.addValidationError(err)
	}
	return e
}

func (e *API) bindWith(destination any, requestBinding binding.Binding) error {
	switch requestBinding {
	case nil:
		return e.Context.ShouldBindUri(destination)
	case binding.Query:
		return e.Context.ShouldBindWith(destination, binding.Query)
	default:
		return e.Context.ShouldBindWith(destination, requestBinding)
	}
}

func (e *API) addValidationError(err error) *API {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return e.AddError(err)
	}
	translator, translatorErr := transInit(e.getAcceptLanguage())
	if translatorErr != nil {
		return e.AddError(errors.Join(err, fmt.Errorf("initialize validation translator: %w", translatorErr)))
	}
	translated := validationErrors.Translate(translator)
	keys := make([]string, 0, len(translated))
	for key := range translated {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	messages := make([]string, 0, len(keys))
	for _, key := range keys {
		messages = append(messages, key+":"+translated[key])
	}
	if len(messages) == 0 {
		return e.AddError(err)
	}
	return e.AddError(errors.New(strings.Join(messages, ",")))
}

func normalizeBindings(request *http.Request, candidates []binding.Binding) []binding.Binding {
	if request == nil {
		return nil
	}

	method := request.Method
	bodyAllowed := method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
	contentType := request.Header.Get("Content-Type")
	if index := strings.IndexByte(contentType, ';'); index >= 0 {
		contentType = contentType[:index]
	}
	selected := binding.Default(method, strings.TrimSpace(contentType))

	result := make([]binding.Binding, 0, len(candidates))
	bodyCandidates := make([]binding.Binding, 0, 3)
	formCandidate := false
	seen := make(map[string]bool)
	appendUnique := func(item binding.Binding) {
		key := bindingName(item)
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, item)
	}

	for _, candidate := range candidates {
		switch candidate {
		case nil:
			appendUnique(nil)
		case binding.Query:
			appendUnique(binding.Query)
		case binding.Form:
			formCandidate = true
		case binding.JSON, binding.XML, binding.YAML:
			bodyCandidates = append(bodyCandidates, candidate)
		default:
			appendUnique(candidate)
		}
	}

	// Gin's query binder uses the `form` tag. Apply it before a body binder so
	// form-tagged query parameters remain available on POST/PUT/PATCH requests.
	if formCandidate {
		appendUnique(binding.Query)
	}
	if !bodyAllowed {
		return result
	}
	if formCandidate && (sameBinding(selected, binding.Form) || sameBinding(selected, binding.FormMultipart)) {
		appendUnique(selected)
	}

	var bodyBinding binding.Binding
	for _, candidate := range bodyCandidates {
		if sameBinding(candidate, selected) {
			bodyBinding = candidate
			break
		}
	}
	if bodyBinding == nil && len(bodyCandidates) == 1 && strings.TrimSpace(contentType) == "" {
		// Preserve the historical ability to bind a single declared body format
		// when clients omit Content-Type, without misreading an explicitly
		// different media type.
		bodyBinding = bodyCandidates[0]
	}
	if bodyBinding != nil {
		appendUnique(bodyBinding)
	}
	return result
}

func bindingName(requestBinding binding.Binding) string {
	if requestBinding == nil {
		return "uri"
	}
	return requestBinding.Name()
}

func sameBinding(left, right binding.Binding) bool {
	return bindingName(left) == bindingName(right)
}

// Err writes an error response.
func (e *API) Err(code int, msg ...string) {
	Default.Error(e.Context, code, e.Error, msg...)
}

// OK writes a success response.
func (e *API) OK(data any, _ ...string) {
	Default.OK(e.Context, data)
}

// PageOK writes a paginated success response.
func (e *API) PageOK(result any, count int64, pageIndex int64, pageSize int64, _ ...string) {
	Default.PageOK(e.Context, result, count, pageIndex, pageSize)
}

func (e *API) getAcceptLanguage() string {
	if e == nil || e.Context == nil {
		return DefaultLanguage
	}
	languages := language.ParseAcceptLanguage(e.Context.GetHeader("Accept-Language"), nil)
	if len(languages) == 0 {
		return DefaultLanguage
	}
	return languages[0]
}

// GetRequestLogger returns the request logger or creates one from the trace ID.
func GetRequestLogger(c *gin.Context) *slog.Logger {
	if c == nil {
		return slog.Default()
	}
	if loggerValue, exists := c.Get(pkg.LoggerKey); exists {
		if logger, ok := loggerValue.(*slog.Logger); ok && logger != nil {
			return logger
		}
	}
	requestID := pkg.GenerateMsgIDFromContext(c)
	return slog.Default().With(strings.ToLower(pkg.TrafficKey), requestID)
}

func methodNotAllowed(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"success":      false,
		"errorMessage": "Method Not Allowed",
	})
}
