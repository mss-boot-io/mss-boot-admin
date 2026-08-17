package dto

// ThemeOverrides contains only values explicitly configured at one theme
// scope. Nil fields inherit from the next lower-precedence scope.
type ThemeOverrides struct {
	NavTheme     *string `json:"navTheme,omitempty"`
	Layout       *string `json:"layout,omitempty"`
	ContentWidth *string `json:"contentWidth,omitempty"`
	FixedHeader  *bool   `json:"fixedHeader,omitempty"`
	FixSiderbar  *bool   `json:"fixSiderbar,omitempty"`
	ColorWeak    *bool   `json:"colorWeak,omitempty"`
	ColorPrimary *string `json:"colorPrimary,omitempty"`
}

// ThemeResource is the sole V6 theme representation. ThemeOverrides is
// embedded so the seven supported values remain flat while _meta carries the
// authoritative scope revision used for reconciliation.
type ThemeResource struct {
	ThemeOverrides
	Meta ThemeResourceMeta `json:"_meta"`
}

type ThemeResourceMeta struct {
	Version  int    `json:"v"`
	Scope    string `json:"scope"`
	Revision string `json:"revision"`
}

// ThemeRevisionConflictResponse documents the structured 412 payload returned
// when an If-Match revision is stale. Current is the authoritative resource a
// client can reconcile before retrying.
type ThemeRevisionConflictResponse struct {
	Success      bool                              `json:"success"`
	Status       string                            `json:"status"`
	Code         int                               `json:"code"`
	ErrorCode    string                            `json:"errorCode"`
	ErrorMessage string                            `json:"errorMessage"`
	TraceID      string                            `json:"traceId"`
	Data         ThemeRevisionConflictResponseData `json:"data"`
}

type ThemeRevisionConflictResponseData struct {
	Current *ThemeResource `json:"current"`
}
