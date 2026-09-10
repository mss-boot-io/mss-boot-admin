package litellmops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Environment configuration for the LiteLLM Admin API client.
const (
	EnvLiteLLMBaseURL   = "LITELLMOPS_LITELLM_BASE_URL"
	EnvLiteLLMMasterKey = "LITELLMOPS_LITELLM_MASTER_KEY"
)

// Client talks to LiteLLM Admin API. Reads power snapshots and bills; writes
// are limited to recharge (/user/update, /key/update). The master key stays
// in process memory only.
type Client struct {
	baseURL    string
	masterKey  string
	httpClient *http.Client
}

// UpstreamError is intentionally safe to return to an operator. It never
// includes the upstream response body, authorization header, or request
// payload. Uncertain is true when a write may have reached LiteLLM even though
// the response could not be read completely.
type UpstreamError struct {
	Path       string
	StatusCode int
	RequestID  string
	Uncertain  bool
	Cause      error
}

func (err *UpstreamError) Error() string {
	if err == nil {
		return "litellmops upstream request failed"
	}
	message := fmt.Sprintf("litellmops LiteLLM %s request failed", err.Path)
	if err.StatusCode != 0 {
		message += fmt.Sprintf(" with status %d", err.StatusCode)
	}
	if err.RequestID != "" {
		message += " (request_id=" + err.RequestID + ")"
	}
	return message
}

func (err *UpstreamError) Unwrap() error { return err.Cause }

func upstreamResultUncertain(err error) bool {
	var upstream *UpstreamError
	return errors.As(err, &upstream) && upstream.Uncertain
}

// NewClient validates explicit configuration.
func NewClient(baseURL, masterKey string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	masterKey = strings.TrimSpace(masterKey)
	if baseURL == "" || masterKey == "" {
		return nil, errors.New("litellmops LiteLLM base URL and master key are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("litellmops LiteLLM base URL must be an absolute http(s) origin")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: baseURL, masterKey: masterKey, httpClient: httpClient}, nil
}

// NewClientFromEnv builds the client from the deployment environment.
func NewClientFromEnv() (*Client, error) {
	return NewClient(os.Getenv(EnvLiteLLMBaseURL), os.Getenv(EnvLiteLLMMasterKey), nil)
}

// RemoteUser is the tolerated shape of one LiteLLM /user/list item.
type RemoteUser struct {
	UserID         string
	Email          string
	Role           string
	Models         []string
	MaxBudget      *float64
	BudgetDuration *string
	BudgetResetAt  *time.Time
	Spend          float64
	TPMLimit       *int64
	RPMLimit       *int64
	Blocked        bool
}

// RemoteKey is the tolerated shape of one LiteLLM /key/list item. TokenHash
// holds the complete hash only transiently; snapshots persist only Prefix().
type RemoteKey struct {
	TokenHash   string
	Alias       string
	UserID      string
	MaxBudget   *float64
	Spend       float64
	TPMLimit    *int64
	RPMLimit    *int64
	MaxParallel *int
	Expires     *time.Time
	CreatedAt   *time.Time
	Models      []string
	Blocked     bool
}

// keyHashPrefixLength bounds what the snapshot may persist.
const keyHashPrefixLength = 16

// Prefix returns the only key material the module is allowed to store.
func (key RemoteKey) Prefix() string {
	if len(key.TokenHash) > keyHashPrefixLength {
		return key.TokenHash[:keyHashPrefixLength]
	}
	return key.TokenHash
}

// IsSession reports the LiteLLM UI session-key shape documented 2026-09-08:
// no alias, $1 budget, about 24h validity.
func (key RemoteKey) IsSession() bool {
	if strings.TrimSpace(key.Alias) != "" || key.MaxBudget == nil || *key.MaxBudget != 1 ||
		key.Expires == nil || key.CreatedAt == nil {
		return false
	}
	duration := key.Expires.Sub(*key.CreatedAt)
	return duration > 0 && duration <= 25*time.Hour
}

func (client *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := client.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("litellmops build LiteLLM request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.masterKey)
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("litellmops call LiteLLM %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fmt.Errorf("litellmops read LiteLLM %s response: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("litellmops LiteLLM %s returned status %d", path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("litellmops decode LiteLLM %s response: %w", path, err)
	}
	return nil
}

func (client *Client) postJSON(ctx context.Context, path string, payload any, out any) error {
	return client.doJSON(ctx, http.MethodPost, path, nil, payload, out)
}

func (client *Client) doJSON(ctx context.Context, method, path string, query url.Values, payload any, out any) error {
	endpoint := client.baseURL + path
	displayPath := safeUpstreamPath(path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("litellmops encode LiteLLM %s request: %w", path, err)
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("litellmops build LiteLLM request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.masterKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return &UpstreamError{Path: displayPath, Uncertain: method != http.MethodGet, Cause: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return &UpstreamError{Path: displayPath, RequestID: safeRequestID(resp.Header), Uncertain: method != http.MethodGet, Cause: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &UpstreamError{Path: displayPath, StatusCode: resp.StatusCode, RequestID: safeRequestID(resp.Header), Uncertain: method != http.MethodGet && resp.StatusCode >= http.StatusInternalServerError}
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return &UpstreamError{Path: displayPath, StatusCode: resp.StatusCode, RequestID: safeRequestID(resp.Header), Uncertain: method != http.MethodGet, Cause: err}
	}
	return nil
}

func safeUpstreamPath(path string) string {
	if strings.HasPrefix(path, "/key/") && strings.HasSuffix(path, "/reset_spend") {
		return "/key/{key}/reset_spend"
	}
	return path
}

func safeRequestID(header http.Header) string {
	for _, key := range []string{"X-Request-ID", "Request-ID", "Trace-ID"} {
		value := strings.TrimSpace(header.Get(key))
		if value == "" {
			continue
		}
		if len(value) > 96 {
			value = value[:96]
		}
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_.:/", r) {
				return r
			}
			return -1
		}, value)
	}
	return ""
}

// GetUser loads one user from /user/info.
func (client *Client) GetUser(ctx context.Context, userID string) (*RemoteUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("litellmops user id is required")
	}
	var envelope map[string]json.RawMessage
	if err := client.getJSON(ctx, "/user/info", url.Values{"user_id": {userID}}, &envelope); err != nil {
		return nil, err
	}
	item := map[string]any{}
	for _, key := range []string{"user_info", "user", "data"} {
		raw, ok := envelope[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, &item); err == nil && item["user_id"] != nil {
			break
		}
		item = map[string]any{}
	}
	if item["user_id"] == nil {
		if err := json.Unmarshal(mustRaw(envelope), &item); err != nil || item["user_id"] == nil {
			return nil, errors.New("litellmops LiteLLM user info is missing user_id")
		}
	}
	user := RemoteUser{
		UserID:         mapString(item, "user_id"),
		Email:          mapString(item, "user_email", "email"),
		Role:           mapString(item, "user_role", "role"),
		Models:         mapStringSlice(item, "models"),
		MaxBudget:      mapFloatPtr(item, "max_budget"),
		BudgetDuration: mapStringPtr(item, "budget_duration"),
		BudgetResetAt:  mapTimePtr(item, "budget_reset_at"),
		Spend:          mapFloat(item, "spend"),
		TPMLimit:       mapInt64Ptr(item, "tpm_limit"),
		RPMLimit:       mapInt64Ptr(item, "rpm_limit"),
		Blocked:        mapBool(item, "blocked"),
	}
	if user.UserID == "" {
		return nil, errors.New("litellmops LiteLLM user info is missing user_id")
	}
	return &user, nil
}

func mustRaw(envelope map[string]json.RawMessage) []byte {
	body := map[string]any{}
	for key, raw := range envelope {
		var value any
		if err := json.Unmarshal(raw, &value); err == nil {
			body[key] = value
		}
	}
	encoded, _ := json.Marshal(body)
	return encoded
}

// UpdateUserBudget sets max_budget through /user/update.
func (client *Client) UpdateUserBudget(ctx context.Context, userID string, maxBudget float64) error {
	return client.postJSON(ctx, "/user/update", map[string]any{
		"user_id":    userID,
		"max_budget": maxBudget,
	}, nil)
}

// UpdateKeyBudget sets max_budget through /key/update. keyToken is the
// hashed token LiteLLM already stores — never a newly minted secret.
func (client *Client) UpdateKeyBudget(ctx context.Context, keyToken string, maxBudget float64) error {
	return client.postJSON(ctx, "/key/update", map[string]any{
		"key":        keyToken,
		"max_budget": maxBudget,
	}, nil)
}

// listItems pages a list endpoint and returns tolerated per-item maps.
// LiteLLM paginates with "page" plus an endpoint-specific size parameter
// ("page_size" for users, "size" for keys).
func (client *Client) listItems(
	ctx context.Context,
	path string,
	sizeParam string,
	extraQuery url.Values,
	collectionKeys ...string,
) ([]map[string]any, error) {
	const pageSize = 100
	var items []map[string]any
	for page := 1; page <= 100; page++ {
		var envelope map[string]json.RawMessage
		query := url.Values{"page": {fmt.Sprint(page)}}
		query.Set(sizeParam, fmt.Sprint(pageSize))
		for key, values := range extraQuery {
			query[key] = values
		}
		if err := client.getJSON(ctx, path, query, &envelope); err != nil {
			return nil, err
		}
		var batch []map[string]any
		for _, key := range collectionKeys {
			raw, ok := envelope[key]
			if !ok {
				continue
			}
			if err := json.Unmarshal(raw, &batch); err == nil && batch != nil {
				break
			}
		}
		if batch == nil {
			// Tolerate a bare array or a single-page object without a wrapper.
			for _, key := range collectionKeys {
				if raw, ok := envelope[key]; ok {
					var single map[string]any
					if err := json.Unmarshal(raw, &single); err == nil && single != nil {
						batch = []map[string]any{single}
						break
					}
				}
			}
		}
		if batch == nil {
			return nil, fmt.Errorf("litellmops LiteLLM %s response has no %v collection", path, collectionKeys)
		}
		items = append(items, batch...)
		if len(batch) < pageSize {
			return items, nil
		}
	}
	return items, errors.New("litellmops LiteLLM list exceeded the 100-page safety bound")
}

// ListUsers returns every LiteLLM internal user (paginated /user/list).
func (client *Client) ListUsers(ctx context.Context) ([]RemoteUser, error) {
	items, err := client.listItems(ctx, "/user/list", "page_size", nil, "users", "data")
	if err != nil {
		return nil, err
	}
	users := make([]RemoteUser, 0, len(items))
	for _, item := range items {
		user := RemoteUser{
			UserID:         mapString(item, "user_id"),
			Email:          mapString(item, "user_email", "email"),
			Role:           mapString(item, "user_role", "role"),
			Models:         mapStringSlice(item, "models"),
			MaxBudget:      mapFloatPtr(item, "max_budget"),
			BudgetDuration: mapStringPtr(item, "budget_duration"),
			BudgetResetAt:  mapTimePtr(item, "budget_reset_at"),
			Spend:          mapFloat(item, "spend"),
			TPMLimit:       mapInt64Ptr(item, "tpm_limit"),
			RPMLimit:       mapInt64Ptr(item, "rpm_limit"),
			Blocked:        mapBool(item, "blocked"),
		}
		if user.UserID == "" {
			return nil, errors.New("litellmops LiteLLM user list item is missing user_id")
		}
		users = append(users, user)
	}
	return users, nil
}

// ListKeys returns every LiteLLM virtual key (paginated /key/list).
func (client *Client) ListKeys(ctx context.Context) ([]RemoteKey, error) {
	// return_full_object is required: the default response is a bare list of
	// key strings without budget/limit/expiry fields.
	items, err := client.listItems(ctx, "/key/list", "size", url.Values{"return_full_object": {"true"}}, "keys", "data")
	if err != nil {
		return nil, err
	}
	keys := make([]RemoteKey, 0, len(items))
	for _, item := range items {
		key := RemoteKey{
			TokenHash:   mapString(item, "token", "key"),
			Alias:       mapString(item, "key_alias", "alias"),
			UserID:      mapString(item, "user_id"),
			MaxBudget:   mapFloatPtr(item, "max_budget"),
			Spend:       mapFloat(item, "spend"),
			TPMLimit:    mapInt64Ptr(item, "tpm_limit"),
			RPMLimit:    mapInt64Ptr(item, "rpm_limit"),
			MaxParallel: mapIntPtr(item, "max_parallel_requests"),
			Expires:     mapTimePtr(item, "expires"),
			CreatedAt:   mapTimePtr(item, "created_at"),
			Models:      mapStringSlice(item, "models"),
			Blocked:     mapBool(item, "blocked"),
		}
		if key.TokenHash == "" {
			return nil, errors.New("litellmops LiteLLM key list item is missing token hash")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func mapString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func mapStringPtr(item map[string]any, keys ...string) *string {
	text := mapString(item, keys...)
	if text == "" {
		return nil
	}
	return &text
}

func mapFloat(item map[string]any, keys ...string) float64 {
	if value := mapFloatPtr(item, keys...); value != nil {
		return *value
	}
	return 0
}

func mapFloatPtr(item map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		switch number := value.(type) {
		case float64:
			return &number
		case string:
			var parsed float64
			if _, err := fmt.Sscanf(number, "%g", &parsed); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func mapInt64Ptr(item map[string]any, keys ...string) *int64 {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		if number, ok := value.(float64); ok {
			converted := int64(number)
			return &converted
		}
	}
	return nil
}

func mapIntPtr(item map[string]any, keys ...string) *int {
	if value := mapInt64Ptr(item, keys...); value != nil {
		converted := int(*value)
		return &converted
	}
	return nil
}

func mapStringSlice(item map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		list, ok := value.([]any)
		if !ok {
			continue
		}
		out := make([]string, 0, len(list))
		for _, entry := range list {
			if text, ok := entry.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	}
	return nil
}

func mapBool(item map[string]any, keys ...string) bool {
	for _, key := range keys {
		switch value := item[key].(type) {
		case bool:
			return value
		case string:
			return strings.EqualFold(strings.TrimSpace(value), "true") || value == "1"
		case float64:
			return value != 0
		}
	}
	return false
}

// mapTimePtr tolerates RFC3339 and naive SQL timestamps (assumed UTC).
func mapTimePtr(item map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := item[key]
		if !ok || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		text = strings.TrimSpace(text)
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999999",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.999999",
			"2006-01-02 15:04:05",
		} {
			if parsed, err := time.Parse(layout, text); err == nil {
				utc := parsed.UTC()
				return &utc
			}
		}
	}
	return nil
}
