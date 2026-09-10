package litellmops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const defaultNewKeyTPM int64 = 600_000

type userMutationRequest struct {
	UserID            string   `json:"user_id"`
	Email             string   `json:"email"`
	UserRole          string   `json:"user_role"`
	Models            []string `json:"models"`
	MaxBudget         *float64 `json:"max_budget"`
	MaxBudgetUSDMicro *int64   `json:"max_budget_usd_micro"`
	BudgetDuration    *string  `json:"budget_duration"`
	TPMLimit          *int64   `json:"tpm_limit"`
	RPMLimit          *int64   `json:"rpm_limit"`
	Blocked           *bool    `json:"blocked"`
}

type keyMutationRequest struct {
	UserID              string   `json:"user_id"`
	UserEmail           string   `json:"user_email"`
	Alias               string   `json:"alias"`
	Models              []string `json:"models"`
	MaxBudget           *float64 `json:"max_budget"`
	MaxBudgetUSDMicro   *int64   `json:"max_budget_usd_micro"`
	TPMLimit            *int64   `json:"tpm_limit"`
	RPMLimit            *int64   `json:"rpm_limit"`
	MaxParallelRequests *int     `json:"max_parallel_requests"`
	Duration            string   `json:"duration"`
	Expires             string   `json:"expires"`
}

type resetSpendRequest struct {
	ResetToUSDMicro int64 `json:"reset_to_usd_micro"`
}
type modelActionRequest struct {
	ModelID string `json:"model_id"`
}

type GatewayModel struct {
	ID              string `json:"id"`
	ModelName       string `json:"model_name"`
	Mode            string `json:"mode"`
	LiteLLMProvider string `json:"litellm_provider"`
	BaseModel       string `json:"base_model"`
}

type GatewayModelsPage struct {
	Items []GatewayModel `json:"items"`
	Total int            `json:"total"`
}

type GatewayHealth struct {
	Ready  bool   `json:"ready"`
	Status string `json:"status"`
	DB     string `json:"db"`
}

type oneTimeKeyResponse struct {
	RawKey              string   `json:"raw_key,omitempty"`
	KeyAlias            string   `json:"key_alias,omitempty"`
	UserID              string   `json:"user_id,omitempty"`
	Models              []string `json:"models,omitempty"`
	MaxBudget           *float64 `json:"max_budget,omitempty"`
	TPMLimit            *int64   `json:"tpm_limit,omitempty"`
	RPMLimit            *int64   `json:"rpm_limit,omitempty"`
	MaxParallelRequests *int     `json:"max_parallel_requests,omitempty"`
	Expires             *string  `json:"expires,omitempty"`
}

func optionalUSD(micro *int64, legacy *float64) (*float64, error) {
	if micro == nil && legacy == nil {
		return nil, nil
	}
	if micro != nil && *micro < 0 {
		return nil, ErrInvalidRecharge
	}
	if legacy != nil {
		legacyMicro, err := usdToMicro(*legacy)
		if err != nil {
			return nil, err
		}
		if micro != nil && legacyMicro != *micro {
			return nil, ErrInvalidRecharge
		}
		value := microToUSD(legacyMicro)
		return &value, nil
	}
	converted := microToUSD(*micro)
	return &converted, nil
}

func requestedKeyDuration(request keyMutationRequest) (string, error) {
	if value := strings.TrimSpace(request.Duration); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(request.Expires); value != "" {
		expires, err := time.Parse(time.RFC3339, value)
		if err != nil || !expires.After(time.Now().UTC()) {
			return "", ErrInvalidRecharge
		}
		return strings.TrimSpace(time.Until(expires).Round(time.Second).String()), nil
	}
	return "", nil
}

func (client *Client) createUser(ctx context.Context, request userMutationRequest, invite bool) (map[string]any, error) {
	budget, err := optionalUSD(request.MaxBudgetUSDMicro, request.MaxBudget)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if value := strings.TrimSpace(request.UserID); value != "" {
		payload["user_id"] = value
	}
	if value := strings.ToLower(strings.TrimSpace(request.Email)); value != "" {
		payload["user_email"] = value
	}
	if value := strings.TrimSpace(request.UserRole); value != "" {
		payload["user_role"] = value
	}
	if len(request.Models) > 0 {
		payload["models"] = request.Models
	}
	if budget != nil {
		payload["max_budget"] = *budget
	}
	if request.TPMLimit != nil {
		payload["tpm_limit"] = *request.TPMLimit
	}
	if request.RPMLimit != nil {
		payload["rpm_limit"] = *request.RPMLimit
	}
	if request.BudgetDuration != nil {
		payload["budget_duration"] = strings.TrimSpace(*request.BudgetDuration)
	}
	if invite {
		payload["send_invite_email"] = true
	}
	var out map[string]any
	if err := client.postJSON(ctx, "/user/new", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) updateUser(ctx context.Context, userID string, request userMutationRequest) (map[string]any, error) {
	budget, err := optionalUSD(request.MaxBudgetUSDMicro, request.MaxBudget)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"user_id": userID}
	if value := strings.ToLower(strings.TrimSpace(request.Email)); value != "" {
		payload["user_email"] = value
	}
	if value := strings.TrimSpace(request.UserRole); value != "" {
		payload["user_role"] = value
	}
	if request.Models != nil {
		payload["models"] = request.Models
	}
	if budget != nil {
		payload["max_budget"] = *budget
	}
	if request.TPMLimit != nil {
		payload["tpm_limit"] = *request.TPMLimit
	}
	if request.RPMLimit != nil {
		payload["rpm_limit"] = *request.RPMLimit
	}
	if request.Blocked != nil {
		payload["blocked"] = *request.Blocked
	}
	if request.BudgetDuration != nil {
		payload["budget_duration"] = strings.TrimSpace(*request.BudgetDuration)
	}
	var out map[string]any
	if err := client.postJSON(ctx, "/user/update", payload, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (client *Client) deleteUser(ctx context.Context, userID string) error {
	return client.postJSON(ctx, "/user/delete", map[string]any{"user_ids": []string{userID}}, nil)
}

func (client *Client) issueKey(ctx context.Context, request keyMutationRequest) (*oneTimeKeyResponse, error) {
	budget, err := optionalUSD(request.MaxBudgetUSDMicro, request.MaxBudget)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{}
	if value := strings.TrimSpace(request.UserID); value != "" {
		payload["user_id"] = value
	}
	if value := strings.TrimSpace(request.Alias); value != "" {
		payload["key_alias"] = value
	}
	if request.Models != nil {
		payload["models"] = request.Models
	}
	if budget != nil {
		payload["max_budget"] = *budget
	}
	if request.TPMLimit == nil {
		payload["tpm_limit"] = defaultNewKeyTPM
	} else {
		payload["tpm_limit"] = *request.TPMLimit
	}
	if request.RPMLimit != nil {
		payload["rpm_limit"] = *request.RPMLimit
	}
	if request.MaxParallelRequests != nil {
		payload["max_parallel_requests"] = *request.MaxParallelRequests
	}
	duration, err := requestedKeyDuration(request)
	if err != nil {
		return nil, err
	}
	if duration != "" {
		payload["duration"] = duration
	}
	var raw map[string]any
	if err := client.postJSON(ctx, "/key/generate", payload, &raw); err != nil {
		return nil, err
	}
	return extractOneTimeKey(raw), nil
}

func extractOneTimeKey(raw map[string]any) *oneTimeKeyResponse {
	result := &oneTimeKeyResponse{KeyAlias: mapString(raw, "key_alias"), UserID: mapString(raw, "user_id"), Models: mapStringSlice(raw, "models"), MaxBudget: mapFloatPtr(raw, "max_budget"), TPMLimit: mapInt64Ptr(raw, "tpm_limit"), RPMLimit: mapInt64Ptr(raw, "rpm_limit"), MaxParallelRequests: mapIntPtr(raw, "max_parallel_requests")}
	result.RawKey = mapString(raw, "key", "token")
	if expires := mapString(raw, "expires"); expires != "" {
		result.Expires = &expires
	}
	return result
}

func (client *Client) updateKey(ctx context.Context, key string, request keyMutationRequest) error {
	budget, err := optionalUSD(request.MaxBudgetUSDMicro, request.MaxBudget)
	if err != nil {
		return err
	}
	payload := map[string]any{"key": key}
	if value := strings.TrimSpace(request.Alias); value != "" {
		payload["key_alias"] = value
	}
	if request.Models != nil {
		payload["models"] = request.Models
	}
	if budget != nil {
		payload["max_budget"] = *budget
	}
	if request.TPMLimit != nil {
		payload["tpm_limit"] = *request.TPMLimit
	}
	if request.RPMLimit != nil {
		payload["rpm_limit"] = *request.RPMLimit
	}
	if request.MaxParallelRequests != nil {
		payload["max_parallel_requests"] = *request.MaxParallelRequests
	}
	duration, err := requestedKeyDuration(request)
	if err != nil {
		return err
	}
	if duration != "" {
		payload["duration"] = duration
	}
	return client.postJSON(ctx, "/key/update", payload, nil)
}
func (client *Client) keyAction(ctx context.Context, path, key string) error {
	return client.postJSON(ctx, path, map[string]any{"key": key}, nil)
}
func (client *Client) deleteKey(ctx context.Context, key string) error {
	return client.postJSON(ctx, "/key/delete", map[string]any{"keys": []string{key}}, nil)
}
func (client *Client) rotateKey(ctx context.Context, key string) (*oneTimeKeyResponse, error) {
	var raw map[string]any
	if err := client.postJSON(ctx, "/key/regenerate", map[string]any{"key": key}, &raw); err != nil {
		return nil, err
	}
	return extractOneTimeKey(raw), nil
}
func (client *Client) resetKeySpend(ctx context.Context, key string, resetToMicro int64) error {
	if resetToMicro < 0 {
		return ErrInvalidRecharge
	}
	return client.postJSON(ctx, "/key/"+url.PathEscape(key)+"/reset_spend", map[string]any{"reset_to": microToUSD(resetToMicro)}, nil)
}

func (client *Client) gatewayModels(ctx context.Context) (*GatewayModelsPage, error) {
	var envelope map[string]any
	if err := client.getJSON(ctx, "/model/info", nil, &envelope); err != nil {
		return nil, err
	}
	rows := asObjectSlice(envelope["data"])
	items := make([]GatewayModel, 0, len(rows))
	for _, row := range rows {
		info := asObject(row["model_info"])
		params := asObject(row["litellm_params"])
		items = append(items, GatewayModel{
			ID: mapString(info, "id", "model_id"), ModelName: mapString(row, "model_name"),
			Mode:            mapString(info, "mode"),
			LiteLLMProvider: mapString(info, "litellm_provider", "provider"),
			BaseModel:       mapString(info, "base_model"),
		})
		last := &items[len(items)-1]
		if last.Mode == "" {
			last.Mode = mapString(params, "mode")
		}
		if last.LiteLLMProvider == "" {
			last.LiteLLMProvider = mapString(params, "litellm_provider", "custom_llm_provider")
		}
		if last.BaseModel == "" {
			last.BaseModel = mapString(params, "base_model")
		}
	}
	return &GatewayModelsPage{Items: items, Total: len(items)}, nil
}
func (client *Client) gatewayHealth(ctx context.Context) (*GatewayHealth, error) {
	var out map[string]any
	if err := client.getJSON(ctx, "/health/readiness", nil, &out); err != nil {
		return nil, err
	}
	status, database := mapString(out, "status"), mapString(out, "db")
	ready := strings.EqualFold(status, "healthy") && (database == "" || strings.EqualFold(database, "connected"))
	return &GatewayHealth{Ready: ready, Status: status, DB: database}, nil
}
func (client *Client) modelAction(ctx context.Context, path, modelID string) error {
	return client.postJSON(ctx, path, map[string]any{"model_id": modelID}, nil)
}

func resolveFullKey(ctx context.Context, db *gorm.DB, client *Client, id string) (string, *KeySnapshot, error) {
	var snapshot KeySnapshot
	if err := db.WithContext(ctx).First(&snapshot, "id = ?", id).Error; err != nil {
		return "", nil, err
	}
	keys, err := client.ListKeys(ctx)
	if err != nil {
		return "", nil, err
	}
	var matched string
	for _, key := range keys {
		if key.Prefix() == snapshot.KeyHashPrefix {
			if matched != "" {
				return "", nil, errors.New("litellmops key prefix is ambiguous")
			}
			matched = key.TokenHash
		}
	}
	if matched == "" {
		return "", nil, gorm.ErrRecordNotFound
	}
	return matched, &snapshot, nil
}

func safeMutationDigest(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func beginManagementAttempt(db *gorm.DB, operator, target, step string, request any) (*OperationAttempt, error) {
	attempt := &OperationAttempt{
		ID: newSnapshotID(), OperationType: "management", OperationID: newSnapshotID(), TargetID: target,
		Step: step, Attempt: 1, RequestDigest: safeMutationDigest(request), ResultCode: "started",
		StartedAt: time.Now().UTC(), Operator: strings.TrimSpace(operator),
	}
	if attempt.Operator == "" {
		return nil, errors.New("litellmops management operator is required")
	}
	return attempt, db.Create(attempt).Error
}

func finishManagementAttempt(db *gorm.DB, attempt *OperationAttempt, operationErr error) error {
	result := "completed"
	if operationErr != nil {
		result = "failed"
		if upstreamResultUncertain(operationErr) {
			result = "result_unverified"
		}
	}
	now := time.Now().UTC()
	updated := db.Model(&OperationAttempt{}).Where("id = ? AND result_code = ?", attempt.ID, "started").Updates(map[string]any{"result_code": result, "finished_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return errors.New("litellmops management attempt audit conflict")
	}
	return nil
}

func auditedManagement(db *gorm.DB, operator, target, step string, request any, action func() (any, error)) (any, error) {
	attempt, err := beginManagementAttempt(db, operator, target, step, request)
	if err != nil {
		return nil, err
	}
	out, operationErr := action()
	if auditErr := finishManagementAttempt(db, attempt, operationErr); auditErr != nil {
		return out, auditErr
	}
	return out, operationErr
}
func (handler *requestHandler) managementClient(ctx *gin.Context) (*gorm.DB, *Client, bool) {
	db, ok := handler.database(ctx)
	if !ok {
		return nil, nil, false
	}
	client, ok := handler.litellmClient(ctx)
	return db, client, ok
}
func bestEffortSync(ctx context.Context, db *gorm.DB, client *Client) {
	_, _ = SyncSnapshots(ctx, db, client)
}

func (handler *requestHandler) createUser(ctx *gin.Context) { handler.mutateUserCreate(ctx, false) }
func (handler *requestHandler) inviteUser(ctx *gin.Context) { handler.mutateUserCreate(ctx, true) }
func (handler *requestHandler) mutateUserCreate(ctx *gin.Context, invite bool) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var request userMutationRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	if invite && request.MaxBudgetUSDMicro == nil && request.MaxBudget == nil {
		gift := int64(5_000_000)
		request.MaxBudgetUSDMicro = &gift
	}
	target := strings.TrimSpace(request.UserID)
	if target == "" {
		target = strings.ToLower(strings.TrimSpace(request.Email))
	}
	result, err := auditedManagement(db, handler.operator(ctx), target, "user_create", request, func() (any, error) {
		return client.createUser(ctx.Request.Context(), request, invite)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	out := result.(map[string]any)
	bestEffortSync(ctx.Request.Context(), db, client)
	response := gin.H{"user": out}
	if secret := extractOneTimeKey(out); secret.RawKey != "" {
		response["raw_key"] = secret.RawKey
		delete(out, "key")
		delete(out, "token")
	}
	ctx.JSON(http.StatusCreated, response)
}

func (handler *requestHandler) updateUser(ctx *gin.Context) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var snapshot UserSnapshot
	if err := db.First(&snapshot, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	var request userMutationRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	result, err := auditedManagement(db, handler.operator(ctx), snapshot.UserID, "user_update", request, func() (any, error) {
		return client.updateUser(ctx.Request.Context(), snapshot.UserID, request)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	out := result.(map[string]any)
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.JSON(200, gin.H{"user": out})
}
func (handler *requestHandler) blockUser(ctx *gin.Context)   { handler.setUserBlocked(ctx, true) }
func (handler *requestHandler) unblockUser(ctx *gin.Context) { handler.setUserBlocked(ctx, false) }
func (handler *requestHandler) setUserBlocked(ctx *gin.Context, blocked bool) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var snapshot UserSnapshot
	if err := db.First(&snapshot, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	request := userMutationRequest{Blocked: &blocked}
	result, err := auditedManagement(db, handler.operator(ctx), snapshot.UserID, "user_block", request, func() (any, error) {
		return client.updateUser(ctx.Request.Context(), snapshot.UserID, request)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	out := result.(map[string]any)
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.JSON(200, gin.H{"user": out})
}
func (handler *requestHandler) deleteUser(ctx *gin.Context) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var snapshot UserSnapshot
	if err := db.First(&snapshot, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	_, err := auditedManagement(db, handler.operator(ctx), snapshot.UserID, "user_delete", nil, func() (any, error) {
		return nil, client.deleteUser(ctx.Request.Context(), snapshot.UserID)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.Status(http.StatusNoContent)
}

func (handler *requestHandler) issueKey(ctx *gin.Context) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var request keyMutationRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	request.UserID = strings.TrimSpace(request.UserID)
	request.UserEmail = strings.ToLower(strings.TrimSpace(request.UserEmail))
	if request.UserID == "" && request.UserEmail != "" {
		var users []UserSnapshot
		if err := db.Where("LOWER(email) = ?", request.UserEmail).Limit(2).Find(&users).Error; err != nil {
			writeAPIError(ctx, err)
			return
		}
		if len(users) != 1 {
			writeAPIError(ctx, ErrInvalidRecharge)
			return
		}
		request.UserID = users[0].UserID
	}
	if request.UserID == "" {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	result, err := auditedManagement(db, handler.operator(ctx), request.UserID, "key_issue", request, func() (any, error) {
		return client.issueKey(ctx.Request.Context(), request)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	response := result.(*oneTimeKeyResponse)
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.JSON(http.StatusCreated, response)
}
func (handler *requestHandler) updateKey(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_update", func(client *Client, key string) (any, error) {
		var request keyMutationRequest
		if ctx.ShouldBindJSON(&request) != nil {
			return nil, ErrInvalidRecharge
		}
		return nil, client.updateKey(ctx.Request.Context(), key, request)
	})
}
func (handler *requestHandler) blockKey(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_block", func(client *Client, key string) (any, error) {
		return nil, client.keyAction(ctx.Request.Context(), "/key/block", key)
	})
}
func (handler *requestHandler) unblockKey(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_unblock", func(client *Client, key string) (any, error) {
		return nil, client.keyAction(ctx.Request.Context(), "/key/unblock", key)
	})
}
func (handler *requestHandler) deleteKey(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_delete", func(client *Client, key string) (any, error) {
		return nil, client.deleteKey(ctx.Request.Context(), key)
	})
}
func (handler *requestHandler) rotateKey(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_rotate", func(client *Client, key string) (any, error) { return client.rotateKey(ctx.Request.Context(), key) })
}
func (handler *requestHandler) resetKeySpend(ctx *gin.Context) {
	handler.withResolvedKey(ctx, "key_reset_spend", func(client *Client, key string) (any, error) {
		var request resetSpendRequest
		if ctx.ShouldBindJSON(&request) != nil {
			return nil, ErrInvalidRecharge
		}
		return nil, client.resetKeySpend(ctx.Request.Context(), key, request.ResetToUSDMicro)
	})
}
func (handler *requestHandler) withResolvedKey(ctx *gin.Context, step string, action func(*Client, string) (any, error)) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	key, snapshot, err := resolveFullKey(ctx.Request.Context(), db, client, ctx.Param("id"))
	if err == nil {
		var out any
		out, err = auditedManagement(db, handler.operator(ctx), snapshot.ID, step, nil, func() (any, error) {
			return action(client, key)
		})
		if err == nil {
			bestEffortSync(ctx.Request.Context(), db, client)
			if step == "key_delete" {
				ctx.Status(http.StatusNoContent)
				return
			}
			if out == nil {
				out = gin.H{"status": "ok"}
			}
			ctx.JSON(200, out)
			return
		}
	}
	writeAPIError(ctx, err)
}

func (handler *requestHandler) gatewayModels(ctx *gin.Context) {
	_, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	out, err := client.gatewayModels(ctx.Request.Context())
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, out)
}
func (handler *requestHandler) gatewayHealth(ctx *gin.Context) {
	_, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	out, err := client.gatewayHealth(ctx.Request.Context())
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, out)
}
func (handler *requestHandler) blockModel(ctx *gin.Context)   { handler.setModelBlocked(ctx, true) }
func (handler *requestHandler) unblockModel(ctx *gin.Context) { handler.setModelBlocked(ctx, false) }
func (handler *requestHandler) setModelBlocked(ctx *gin.Context, blocked bool) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	modelID := strings.TrimSpace(ctx.Param("id"))
	if modelID == "" {
		var request modelActionRequest
		if ctx.ShouldBindJSON(&request) == nil {
			modelID = strings.TrimSpace(request.ModelID)
		}
	}
	if modelID == "" {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	path := "/model/block"
	step := "model_block"
	if !blocked {
		path = "/model/unblock"
		step = "model_unblock"
	}
	_, err := auditedManagement(db, handler.operator(ctx), modelID, step, nil, func() (any, error) {
		return nil, client.modelAction(ctx.Request.Context(), path, modelID)
	})
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"model_id": modelID, "blocked": blocked})
}
