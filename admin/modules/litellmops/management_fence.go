package litellmops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	ManagementExecuting        = "executing"
	ManagementResultUnverified = "result_unverified"
	ManagementCompleted        = "completed"
	ManagementFailed           = "failed"
	ManagementResolvedApplied  = "resolved_applied"
	ManagementResolvedNoop     = "resolved_not_applied"
)

var managementConfirmDelays = []time.Duration{0, 200 * time.Millisecond, 800 * time.Millisecond, 2 * time.Second}
var managementUncertainRetryAfter = 2 * time.Minute
var managementStableObservationWindow = 5 * time.Second

var (
	ErrManagementPending    = errors.New("litellmops user has a pending management command")
	ErrManagementUnverified = errors.New("litellmops management command requires reconciliation")
)

type managementCommandResultError struct {
	Command ManagementCommand
	Cause   error
}

func (err *managementCommandResultError) Error() string {
	return "litellmops management result requires reconciliation"
}

func (err *managementCommandResultError) Unwrap() error { return err.Cause }

// ManagementCommand is the durable quarantine record for a user/key mutation.
// ExpectedState contains only typed budget/blocked/existence metadata; it never
// contains a full key, credential, response body, or authorization value.
type ManagementCommand struct {
	ID                   string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	ResolvedAt           *time.Time `gorm:"column:resolved_at" json:"resolved_at"`
	UserID               string     `gorm:"column:user_id;type:varchar(64);not null;index:idx_litellmops_management_pending,priority:1" json:"user_id"`
	UserEmail            string     `gorm:"column:user_email;type:varchar(254);index" json:"user_email"`
	TargetType           string     `gorm:"column:target_type;type:varchar(16);not null" json:"target_type"`
	TargetID             string     `gorm:"column:target_id;type:varchar(128);not null" json:"target_id"`
	Action               string     `gorm:"column:action;type:varchar(64);not null" json:"action"`
	PayloadHash          string     `gorm:"column:payload_hash;type:varchar(64);not null" json:"-"`
	ExpectedState        string     `gorm:"column:expected_state;type:text;not null" json:"-"`
	BeforeStateDigest    string     `gorm:"column:before_state_digest;type:varchar(64)" json:"-"`
	Status               string     `gorm:"column:status;type:varchar(32);not null;index:idx_litellmops_management_pending,priority:2" json:"status"`
	LastErrorCode        string     `gorm:"column:last_error_code;type:varchar(64)" json:"last_error_code"`
	RequiresManualReview bool       `gorm:"column:requires_manual_review;not null;default:false;index" json:"requires_manual_review"`
	LastObservedDigest   string     `gorm:"column:last_observed_digest;type:varchar(64)" json:"-"`
	LastObservedAt       *time.Time `gorm:"column:last_observed_at" json:"last_observed_at"`
	Operator             string     `gorm:"column:operator;type:varchar(128);not null" json:"operator"`
	ResolvedBy           string     `gorm:"column:resolved_by;type:varchar(128)" json:"resolved_by"`
	ResolutionReason     string     `gorm:"column:resolution_reason;type:varchar(512)" json:"resolution_reason"`
	Version              int64      `gorm:"column:version;not null;default:1" json:"version"`
}

func (ManagementCommand) TableName() string { return "litellmops_management_command" }

type managementExpectedState struct {
	Kind               string `json:"kind"`
	Email              string `json:"email,omitempty"`
	KeyPrefix          string `json:"key_prefix,omitempty"`
	MaxBudgetUSDMicro  *int64 `json:"max_budget_usd_micro,omitempty"`
	SpendUSDMicro      *int64 `json:"spend_usd_micro,omitempty"`
	Blocked            *bool  `json:"blocked,omitempty"`
	Deleted            bool   `json:"deleted,omitempty"`
	RequiresManualOnly bool   `json:"requires_manual_only,omitempty"`
}

type managementResolveRequest struct {
	Resolution                string `json:"resolution"`
	Reason                    string `json:"reason"`
	ConfirmAuthoritativeState bool   `json:"confirm_authoritative_state"`
}

type managementExpectedSummary struct {
	Kind              string   `json:"kind"`
	AffectedFields    []string `json:"affected_fields"`
	MaxBudgetUSDMicro *int64   `json:"max_budget_usd_micro,omitempty"`
	SpendUSDMicro     *int64   `json:"spend_usd_micro,omitempty"`
	Blocked           *bool    `json:"blocked,omitempty"`
	Deleted           bool     `json:"deleted,omitempty"`
	ManualReason      string   `json:"manual_reason,omitempty"`
}

type managementCommandResponse struct {
	ManagementCommand
	Expected managementExpectedSummary `json:"expected"`
}

func publicManagementCommand(command ManagementCommand) managementCommandResponse {
	expected, _ := decodeManagementExpected(&command)
	fields := make([]string, 0, 4)
	if expected.MaxBudgetUSDMicro != nil {
		fields = append(fields, "max_budget")
	}
	if expected.SpendUSDMicro != nil {
		fields = append(fields, "spend")
	}
	if expected.Blocked != nil {
		fields = append(fields, "blocked")
	}
	if expected.Deleted || expected.Kind == "user_create" {
		fields = append(fields, "existence")
	}
	manualReason := ""
	if expected.RequiresManualOnly || command.RequiresManualReview {
		manualReason = "authoritative result cannot be determined from the safe typed projection"
		if command.Action == "key_issue" || command.Action == "key_rotate" {
			manualReason = "one-time key material cannot be recovered or verified automatically"
		}
		if command.Action == "user_update" || command.Action == "key_update" {
			fields = append(fields, "profile_or_limits")
		} else if len(fields) == 0 {
			fields = append(fields, "profile_or_limits")
		}
	}
	return managementCommandResponse{ManagementCommand: command, Expected: managementExpectedSummary{
		Kind: expected.Kind, AffectedFields: fields, MaxBudgetUSDMicro: expected.MaxBudgetUSDMicro,
		SpendUSDMicro: expected.SpendUSDMicro, Blocked: expected.Blocked, Deleted: expected.Deleted,
		ManualReason: manualReason,
	}}
}

func pendingManagementStatuses() []string {
	return []string{ManagementExecuting, ManagementResultUnverified}
}

func managementResultUncertain(err error) bool {
	return errors.Is(err, ErrManagementUnverified) || upstreamResultUncertain(err)
}

func managementExpectedJSON(expected managementExpectedState) string {
	raw, _ := json.Marshal(expected)
	return string(raw)
}

func decodeManagementExpected(command *ManagementCommand) (managementExpectedState, error) {
	var expected managementExpectedState
	if err := json.Unmarshal([]byte(command.ExpectedState), &expected); err != nil {
		return expected, fmt.Errorf("litellmops invalid management expected state: %w", err)
	}
	if expected.Kind != "user" && expected.Kind != "user_create" && expected.Kind != "key" {
		return expected, errors.New("litellmops invalid management target type")
	}
	return expected, nil
}

func hasPendingManagement(ctx context.Context, db *gorm.DB, userID, email, excludeID string) (bool, error) {
	query := db.WithContext(ctx).Model(&ManagementCommand{})
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		query = query.Where("user_id = ?", userID)
	} else {
		query = query.Where("user_id = ? OR LOWER(user_email) = ?", userID, email)
	}
	query = query.Where("status IN ?", pendingManagementStatuses())
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count != 0, nil
}

func beginManagementCommand(ctx context.Context, db *gorm.DB, operator, userID, targetType, targetID, action string, request any, expected managementExpectedState, beforeDigest string) (*ManagementCommand, *OperationAttempt, error) {
	now := time.Now().UTC()
	command := &ManagementCommand{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, UserID: strings.TrimSpace(userID),
		UserEmail:  strings.ToLower(strings.TrimSpace(expected.Email)),
		TargetType: targetType, TargetID: strings.TrimSpace(targetID), Action: action,
		PayloadHash: safeMutationDigest(request), ExpectedState: managementExpectedJSON(expected),
		BeforeStateDigest: beforeDigest,
		Status:            ManagementExecuting, RequiresManualReview: expected.RequiresManualOnly,
		Operator: strings.TrimSpace(operator), Version: 1,
	}
	if command.UserID == "" || command.TargetID == "" || command.Operator == "" {
		return nil, nil, ErrInvalidRecharge
	}
	var attempt *OperationAttempt
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rechargePending int64
		if err := tx.Model(&RechargeRecord{}).Where("user_id = ? AND status IN ?", command.UserID, pendingRechargeStatuses()).Count(&rechargePending).Error; err != nil {
			return err
		}
		if rechargePending != 0 {
			return ErrPendingRecharge
		}
		pending, err := hasPendingManagement(ctx, tx, command.UserID, command.UserEmail, "")
		if err != nil {
			return err
		}
		if pending {
			return ErrManagementPending
		}
		if err := tx.Create(command).Error; err != nil {
			return err
		}
		attempt, err = beginManagementAttempt(tx, command.Operator, command.ID, command.TargetID, action, request)
		return err
	})
	return command, attempt, err
}

func saveManagementCommand(ctx context.Context, db *gorm.DB, command *ManagementCommand, values map[string]any) error {
	values["updated_at"] = time.Now().UTC()
	values["version"] = gorm.Expr("version + 1")
	result := db.WithContext(ctx).Model(&ManagementCommand{}).
		Where("id = ? AND version = ?", command.ID, command.Version).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrManagementPending
	}
	return db.WithContext(ctx).First(command, "id = ?", command.ID).Error
}

func finishManagementCommand(ctx context.Context, db *gorm.DB, command *ManagementCommand, attempt *OperationAttempt, operationErr error) error {
	status, code := ManagementCompleted, ""
	if operationErr != nil {
		status, code = ManagementFailed, "upstream_failed"
		if managementResultUncertain(operationErr) {
			status, code = ManagementResultUnverified, "result_unverified"
		}
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveManagementCommand(ctx, tx, command, map[string]any{"status": status, "last_error_code": code}); err != nil {
			return err
		}
		return finishManagementAttempt(tx, attempt, operationErr)
	})
}

func runFencedManagement(ctx context.Context, db *gorm.DB, client *Client, operator, userID, targetType, targetID, action string, request any, expected managementExpectedState, remote func() (any, error)) (any, error) {
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx, db, userID, holder)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrRechargeBusy
	}
	defer releaseUserLease(context.Background(), db, userID, holder)
	probe := &ManagementCommand{UserID: userID, TargetID: targetID}
	beforeDigest := ""
	if expected.Kind != "key" || expected.KeyPrefix != "" {
		_, beforeDigest, err = readManagementExpected(ctx, client, probe, expected)
		if err != nil {
			return nil, err
		}
	}
	command, attempt, err := beginManagementCommand(ctx, db, operator, userID, targetType, targetID, action, request, expected, beforeDigest)
	if err != nil {
		return nil, err
	}
	if err := renewUserLease(ctx, db, userID, holder); err != nil {
		return nil, err
	}
	out, operationErr := remote()
	if operationErr == nil && !expected.RequiresManualOnly && managementExpectedVerifiable(expected) {
		confirmed, _ := confirmManagementExpected(ctx, db, client, command, expected, holder)
		if !confirmed {
			operationErr = ErrManagementUnverified
		}
	}
	if finishErr := finishManagementCommand(ctx, db, command, attempt, operationErr); finishErr != nil {
		return out, finishErr
	}
	if operationErr != nil && managementResultUncertain(operationErr) {
		return out, &managementCommandResultError{Command: *command, Cause: operationErr}
	}
	return out, operationErr
}

func managementExpectedVerifiable(expected managementExpectedState) bool {
	return expected.Kind == "user_create" || expected.Deleted || expected.MaxBudgetUSDMicro != nil || expected.SpendUSDMicro != nil || expected.Blocked != nil
}

func confirmManagementExpected(ctx context.Context, db *gorm.DB, client *Client, command *ManagementCommand, expected managementExpectedState, holder string) (bool, error) {
	var lastErr error
	for _, delay := range managementConfirmDelays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
			}
		}
		if err := renewUserLease(ctx, db, command.UserID, holder); err != nil {
			return false, err
		}
		matched, digest, err := readManagementExpected(ctx, client, command, expected)
		if err != nil {
			lastErr = err
			continue
		}
		if matched && (command.BeforeStateDigest == "" || digest != command.BeforeStateDigest) {
			return true, nil
		}
		lastErr = nil
	}
	return false, lastErr
}

func readManagementExpected(ctx context.Context, client *Client, command *ManagementCommand, expected managementExpectedState) (bool, string, error) {
	observed := map[string]any{"kind": expected.Kind, "exists": false}
	if expected.Kind == "user_create" {
		users, err := client.ListUsers(ctx)
		if err != nil {
			return false, "", err
		}
		matches := 0
		for _, user := range users {
			if expected.Email != "" && strings.EqualFold(user.Email, expected.Email) || expected.Email == "" && user.UserID == command.TargetID {
				matches++
			}
		}
		observed["matches"] = matches
		return matches == 1, safeMutationDigest(observed), nil
	}
	if expected.Kind == "user" {
		user, err := client.GetUser(ctx, command.UserID)
		if err != nil {
			var upstream *UpstreamError
			if expected.Deleted && errors.As(err, &upstream) && upstream.StatusCode == http.StatusNotFound {
				return true, safeMutationDigest(observed), nil
			}
			return false, "", err
		}
		observed["exists"] = true
		if expected.Blocked != nil {
			observed["blocked"] = user.Blocked
		}
		if expected.MaxBudgetUSDMicro != nil && user.MaxBudget != nil {
			micro, err := usdToMicro(*user.MaxBudget)
			if err != nil {
				return false, "", err
			}
			observed["max_budget_usd_micro"] = micro
		}
		matched := !expected.Deleted
		if expected.MaxBudgetUSDMicro != nil {
			matched = matched && user.MaxBudget != nil && observed["max_budget_usd_micro"] == *expected.MaxBudgetUSDMicro
		}
		if expected.Blocked != nil {
			matched = matched && user.Blocked == *expected.Blocked
		}
		return matched, safeMutationDigest(observed), nil
	}
	keys, err := client.ListKeys(ctx)
	if err != nil {
		return false, "", err
	}
	var found *RemoteKey
	for index := range keys {
		if keys[index].Prefix() == expected.KeyPrefix && keys[index].UserID == command.UserID {
			if found != nil {
				return false, "", errors.New("litellmops key prefix is ambiguous")
			}
			found = &keys[index]
		}
	}
	if found == nil {
		return expected.Deleted, safeMutationDigest(observed), nil
	}
	observed["exists"] = true
	if expected.Blocked != nil {
		observed["blocked"] = found.Blocked
	}
	budgetMicro, budgetValid := int64(0), false
	if expected.MaxBudgetUSDMicro != nil && found.MaxBudget != nil {
		budgetMicro, err = usdToMicro(*found.MaxBudget)
		if err != nil {
			return false, "", err
		}
		budgetValid = true
		observed["max_budget_usd_micro"] = budgetMicro
	}
	spendMicro := int64(0)
	if expected.SpendUSDMicro != nil {
		spendMicro, err = usdToMicro(found.Spend)
		if err != nil {
			return false, "", err
		}
		observed["spend_usd_micro"] = spendMicro
	}
	matched := !expected.Deleted
	if expected.MaxBudgetUSDMicro != nil {
		matched = matched && budgetValid && budgetMicro == *expected.MaxBudgetUSDMicro
	}
	if expected.SpendUSDMicro != nil {
		matched = matched && spendMicro == *expected.SpendUSDMicro
	}
	if expected.Blocked != nil {
		matched = matched && found.Blocked == *expected.Blocked
	}
	return matched, safeMutationDigest(observed), nil
}

func resolveManagementCommand(ctx context.Context, db *gorm.DB, command *ManagementCommand, actor, status, reason string) error {
	now := time.Now().UTC()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveManagementCommand(ctx, tx, command, map[string]any{
			"status": status, "last_error_code": "", "resolved_at": now,
			"resolved_by": strings.TrimSpace(actor), "resolution_reason": strings.TrimSpace(reason),
		}); err != nil {
			return err
		}
		attempt, err := beginManagementAttempt(tx, actor, command.ID, command.TargetID, "management_resolve", map[string]any{"resolution": status, "reason": reason})
		if err != nil {
			return err
		}
		return finishManagementAttempt(tx, attempt, nil)
	})
}

func observeManagementCommand(ctx context.Context, db *gorm.DB, command *ManagementCommand, digest string) (bool, error) {
	now := time.Now().UTC()
	same := command.LastObservedDigest != "" && command.LastObservedDigest == digest && command.LastObservedAt != nil
	stable := same && now.Sub(*command.LastObservedAt) >= managementStableObservationWindow
	values := map[string]any{}
	if !same {
		values["last_observed_digest"] = digest
		values["last_observed_at"] = now
	}
	return stable, saveManagementCommand(ctx, db, command, values)
}

func reconcileManagementCommand(ctx context.Context, db *gorm.DB, client *Client, command *ManagementCommand, actor string) (resultErr error) {
	attempt, err := beginManagementAttempt(db.WithContext(ctx), actor, command.ID, command.TargetID, "management_reconcile", nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := finishManagementAttempt(db.WithContext(ctx), attempt, resultErr); err != nil {
			resultErr = err
		}
	}()
	if command.Status != ManagementExecuting && command.Status != ManagementResultUnverified {
		return nil
	}
	now := time.Now().UTC()
	if command.Status == ManagementExecuting && now.Sub(command.CreatedAt) < managementUncertainRetryAfter {
		return ErrManagementPending
	}
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx, db, command.UserID, holder)
	if err != nil {
		return err
	}
	if !acquired {
		return ErrRechargeBusy
	}
	defer releaseUserLease(context.Background(), db, command.UserID, holder)
	if err := db.WithContext(ctx).First(command, "id = ?", command.ID).Error; err != nil {
		return err
	}
	if command.Status != ManagementExecuting && command.Status != ManagementResultUnverified {
		return nil
	}
	pending, err := hasPendingManagement(ctx, db, command.UserID, command.UserEmail, command.ID)
	if err != nil {
		return err
	}
	if pending {
		return ErrManagementPending
	}
	var rechargePending int64
	if err := db.WithContext(ctx).Model(&RechargeRecord{}).Where("user_id = ? AND status IN ?", command.UserID, pendingRechargeStatuses()).Count(&rechargePending).Error; err != nil {
		return err
	}
	if rechargePending != 0 {
		return ErrPendingRecharge
	}
	expected, err := decodeManagementExpected(command)
	if err != nil {
		return err
	}
	matched, digest, err := readManagementExpected(ctx, client, command, expected)
	if err != nil {
		return err
	}
	if expected.RequiresManualOnly || !managementExpectedVerifiable(expected) {
		if _, err := observeManagementCommand(ctx, db, command, digest); err != nil {
			return err
		}
		return ErrManagementUnverified
	}
	if matched && (command.BeforeStateDigest == "" || digest != command.BeforeStateDigest) {
		return resolveManagementCommand(ctx, db, command, actor, ManagementResolvedApplied, "authoritative readback matched expected state")
	}
	stable, err := observeManagementCommand(ctx, db, command, digest)
	if err != nil {
		return err
	}
	if !stable || time.Since(command.CreatedAt) < managementUncertainRetryAfter {
		return ErrManagementUnverified
	}
	if command.BeforeStateDigest != "" && digest == command.BeforeStateDigest {
		return resolveManagementCommand(ctx, db, command, actor, ManagementResolvedNoop, "authoritative state remained at the pre-write value after uncertainty window")
	}
	if err := saveManagementCommand(ctx, db, command, map[string]any{
		"requires_manual_review": true,
		"last_error_code":        "authoritative_state_diverged",
	}); err != nil {
		return err
	}
	return ErrManagementUnverified
}

func (handler *requestHandler) listManagementCommands(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&ManagementCommand{})
	if userID := strings.TrimSpace(ctx.Query("user_id")); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if email := strings.ToLower(strings.TrimSpace(ctx.Query("user_email"))); email != "" {
		query = query.Where("LOWER(user_email) = ?", email)
	}
	if status := strings.TrimSpace(ctx.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if action := strings.TrimSpace(ctx.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	page := bindPage(ctx)
	commands := make([]ManagementCommand, 0, page.PageSize)
	if err := query.Order("created_at DESC").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&commands).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	items := make([]managementCommandResponse, 0, len(commands))
	for _, command := range commands {
		items = append(items, publicManagementCommand(command))
	}
	ctx.JSON(200, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) reconcileManagementCommand(ctx *gin.Context) {
	db, client, ok := handler.managementClient(ctx)
	if !ok {
		return
	}
	var command ManagementCommand
	if err := db.First(&command, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	if err := reconcileManagementCommand(ctx.Request.Context(), db, client, &command, handler.operator(ctx)); err != nil {
		if errors.Is(err, ErrManagementPending) || errors.Is(err, ErrManagementUnverified) {
			_ = db.First(&command, "id = ?", command.ID).Error
			ctx.JSON(http.StatusAccepted, publicManagementCommand(command))
			return
		}
		writeAPIError(ctx, err)
		return
	}
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.JSON(200, publicManagementCommand(command))
}

func (handler *requestHandler) resolveManagementCommand(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var request managementResolveRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	request.Resolution = strings.TrimSpace(request.Resolution)
	request.Reason = strings.TrimSpace(request.Reason)
	if (request.Resolution != "applied" && request.Resolution != "not_applied") || request.Reason == "" || len(request.Reason) > 512 || !request.ConfirmAuthoritativeState {
		writeAPIError(ctx, ErrInvalidRecharge)
		return
	}
	var command ManagementCommand
	if err := db.First(&command, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	if command.Status != ManagementExecuting && command.Status != ManagementResultUnverified {
		writeAPIError(ctx, ErrInvalidOrderState)
		return
	}
	if time.Since(command.CreatedAt) < managementUncertainRetryAfter {
		writeAPIError(ctx, ErrManagementPending)
		return
	}
	expected, err := decodeManagementExpected(&command)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if !expected.RequiresManualOnly && !command.RequiresManualReview {
		writeAPIError(ctx, ErrManagementUnverified)
		return
	}
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx.Request.Context(), db, command.UserID, holder)
	if err != nil || !acquired {
		if err == nil {
			err = ErrRechargeBusy
		}
		writeAPIError(ctx, err)
		return
	}
	defer releaseUserLease(context.Background(), db, command.UserID, holder)
	if err := db.WithContext(ctx.Request.Context()).First(&command, "id = ?", command.ID).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	if command.Status != ManagementExecuting && command.Status != ManagementResultUnverified {
		writeAPIError(ctx, ErrInvalidOrderState)
		return
	}
	pending, err := hasPendingManagement(ctx.Request.Context(), db, command.UserID, command.UserEmail, command.ID)
	if err != nil || pending {
		if err == nil {
			err = ErrManagementPending
		}
		writeAPIError(ctx, err)
		return
	}
	var rechargePending int64
	if err := db.WithContext(ctx.Request.Context()).Model(&RechargeRecord{}).Where("user_id = ? AND status IN ?", command.UserID, pendingRechargeStatuses()).Count(&rechargePending).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	if rechargePending != 0 {
		writeAPIError(ctx, ErrPendingRecharge)
		return
	}
	client, ok := handler.litellmClient(ctx)
	if !ok {
		return
	}
	_, digest, err := readManagementExpected(ctx.Request.Context(), client, &command, expected)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	stable, err := observeManagementCommand(ctx.Request.Context(), db, &command, digest)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if !stable {
		writeAPIError(ctx, ErrManagementUnverified)
		return
	}
	status := ManagementResolvedApplied
	if request.Resolution == "not_applied" {
		status = ManagementResolvedNoop
	}
	if err := resolveManagementCommand(ctx.Request.Context(), db, &command, handler.operator(ctx), status, request.Reason); err != nil {
		writeAPIError(ctx, err)
		return
	}
	bestEffortSync(ctx.Request.Context(), db, client)
	ctx.JSON(200, publicManagementCommand(command))
}
