package litellmops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// Current deployment upperbound_key_generate_params.max_budget is $1000.
	KeyBudgetCeilingMicro int64 = 1_000_000_000
	minRechargeMicro      int64 = 10_000
	maxRechargeMicro      int64 = 10_000_000_000
	rechargeLeaseDuration       = 5 * time.Minute

	RechargeApproved          = "approved"
	RechargeExecuting         = "executing"
	RechargeAppliedUnverified = "applied_unverified"
	RechargeCompleted         = "completed"
	RechargeRetryableFailed   = "retryable_failed"
	RechargeTerminalFailed    = "terminal_failed"
	RechargeReconcileRequired = "reconcile_required"
)

var rechargeConfirmDelays = []time.Duration{0, 200 * time.Millisecond, 800 * time.Millisecond, 2 * time.Second}
var rechargeUncertainRetryAfter = 2 * time.Minute
var rechargeStableObservationWindow = 5 * time.Second

var (
	ErrInvalidRecharge     = errors.New("litellmops invalid recharge")
	ErrIdempotencyConflict = errors.New("litellmops idempotency conflict")
	ErrRechargeBusy        = errors.New("litellmops recharge already executing for user")
	ErrPendingRecharge     = errors.New("litellmops user has a pending recharge")
	ErrUnlimitedBudget     = errors.New("litellmops unlimited user budget cannot be recharged")
	ErrReconcileRequired   = errors.New("litellmops recharge requires reconciliation")
)

// RechargeRequest keeps amount for the existing UI while amount_usd_micro is
// the canonical lossless representation used by sales integrations.
type RechargeRequest struct {
	Amount         float64 `json:"amount"`
	AmountUSDMicro int64   `json:"amount_usd_micro"`
	Reason         string  `json:"reason"`
	RaiseKeys      bool    `json:"raise_keys"`
	IdempotencyKey string  `json:"idempotency_key"`
	Source         string  `json:"-"`
	SourceRef      string  `json:"-"`
}

// RechargeRecord is the durable command and audit source of truth. Float
// fields are compatibility projections; decisions use integer micro USD.
type RechargeRecord struct {
	ID                      string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt               time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt               time.Time  `gorm:"column:updated_at" json:"updated_at"`
	CompletedAt             *time.Time `gorm:"column:completed_at" json:"completed_at"`
	UserID                  string     `gorm:"column:user_id;type:varchar(64);not null;uniqueIndex:ux_litellmops_recharge_idempotency,priority:1" json:"user_id"`
	Email                   string     `gorm:"column:email;type:varchar(254)" json:"email"`
	Amount                  float64    `gorm:"column:amount;not null;default:0" json:"amount"`
	BeforeBudget            float64    `gorm:"column:before_budget;not null;default:0" json:"before_budget"`
	AfterBudget             float64    `gorm:"column:after_budget;not null;default:0" json:"after_budget"`
	AmountUSDMicro          int64      `gorm:"column:amount_usd_micro;not null;default:0" json:"amount_usd_micro"`
	BeforeBudgetUSDMicro    *int64     `gorm:"column:before_budget_usd_micro" json:"before_budget_usd_micro"`
	TargetAfterUSDMicro     *int64     `gorm:"column:target_after_usd_micro" json:"target_after_usd_micro"`
	RaiseKeys               bool       `gorm:"column:raise_keys;not null;default:false" json:"raise_keys"`
	KeysUpdated             string     `gorm:"column:keys_updated;type:text" json:"keys_updated"`
	Operator                string     `gorm:"column:operator;type:varchar(128);not null" json:"operator"`
	Reason                  string     `gorm:"column:reason;type:varchar(512)" json:"reason"`
	IdempotencyKey          string     `gorm:"column:idempotency_key;type:varchar(128);not null;uniqueIndex:ux_litellmops_recharge_idempotency,priority:2" json:"idempotency_key"`
	PayloadHash             string     `gorm:"column:payload_hash;type:varchar(64);not null;default:''" json:"payload_hash"`
	Source                  string     `gorm:"column:source;type:varchar(32);not null;default:'manual'" json:"source"`
	SourceRef               string     `gorm:"column:source_ref;type:varchar(128)" json:"source_ref"`
	Status                  string     `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	LastErrorCode           string     `gorm:"column:last_error_code;type:varchar(64)" json:"last_error_code"`
	UncertainSince          *time.Time `gorm:"column:uncertain_since;index" json:"uncertain_since"`
	LastObservedUSDMicro    *int64     `gorm:"column:last_observed_usd_micro" json:"-"`
	LastObservedAt          *time.Time `gorm:"column:last_observed_at" json:"-"`
	LastObservedKeyPrefix   string     `gorm:"column:last_observed_key_prefix;type:varchar(16)" json:"-"`
	LastObservedKeyUSDMicro *int64     `gorm:"column:last_observed_key_usd_micro" json:"-"`
	LastObservedKeyAt       *time.Time `gorm:"column:last_observed_key_at" json:"-"`
	Version                 int64      `gorm:"column:version;not null;default:1" json:"version"`
	AuditOperator           string     `gorm:"-" json:"-"`
}

func (RechargeRecord) TableName() string { return "litellmops_recharge" }

type UserLease struct {
	UserID     string    `gorm:"column:user_id;type:varchar(64);primaryKey" json:"-"`
	Holder     string    `gorm:"column:holder;type:varchar(64);not null" json:"-"`
	LeaseUntil time.Time `gorm:"column:lease_until;not null;index" json:"-"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null" json:"-"`
}

func (UserLease) TableName() string { return "litellmops_user_lease" }

// OperationAttempt excludes response bodies, authorization values, and full
// virtual keys by construction.
type OperationAttempt struct {
	ID            string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	OperationType string     `gorm:"column:operation_type;type:varchar(32);not null;index:idx_litellmops_attempt_operation,priority:1;uniqueIndex:ux_litellmops_attempt_number,priority:1" json:"operation_type"`
	OperationID   string     `gorm:"column:operation_id;type:varchar(64);not null;index:idx_litellmops_attempt_operation,priority:2;uniqueIndex:ux_litellmops_attempt_number,priority:2" json:"operation_id"`
	TargetID      string     `gorm:"column:target_id;type:varchar(128);index" json:"target_id"`
	Step          string     `gorm:"column:step;type:varchar(64);not null;uniqueIndex:ux_litellmops_attempt_number,priority:3" json:"step"`
	Attempt       int        `gorm:"column:attempt;not null;uniqueIndex:ux_litellmops_attempt_number,priority:4" json:"attempt"`
	RequestDigest string     `gorm:"column:request_digest;type:varchar(64)" json:"request_digest"`
	ResultCode    string     `gorm:"column:result_code;type:varchar(64)" json:"result_code"`
	StartedAt     time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at" json:"finished_at"`
	Operator      string     `gorm:"column:operator;type:varchar(128);not null" json:"operator"`
}

func (OperationAttempt) TableName() string { return "litellmops_operation_attempt" }

func usdToMicro(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > float64(math.MaxInt64)/1_000_000 {
		return 0, ErrInvalidRecharge
	}
	return int64(math.Round(value * 1_000_000)), nil
}

func microToUSD(value int64) float64 { return float64(value) / 1_000_000 }

func rechargeAmount(request RechargeRequest) (int64, error) {
	amount := request.AmountUSDMicro
	if request.Amount != 0 {
		legacy, err := usdToMicro(request.Amount)
		if err != nil {
			return 0, err
		}
		if amount != 0 && amount != legacy {
			return 0, fmt.Errorf("%w: amount fields disagree", ErrInvalidRecharge)
		}
		amount = legacy
	}
	if amount < minRechargeMicro || amount > maxRechargeMicro {
		return 0, fmt.Errorf("%w: amount_usd_micro must be between %d and %d", ErrInvalidRecharge, minRechargeMicro, maxRechargeMicro)
	}
	return amount, nil
}

func rechargePayloadHash(userID string, amount int64, raiseKeys bool) string {
	payload, _ := json.Marshal(struct {
		UserID    string `json:"user_id"`
		Amount    int64  `json:"amount_usd_micro"`
		RaiseKeys bool   `json:"raise_keys"`
	}{strings.TrimSpace(userID), amount, raiseKeys})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func reserveRecharge(ctx context.Context, db *gorm.DB, snapshot UserSnapshot, request RechargeRequest, operator string) (*RechargeRecord, bool, error) {
	amount, err := rechargeAmount(request)
	if err != nil {
		return nil, false, err
	}
	idempotency := strings.TrimSpace(request.IdempotencyKey)
	if idempotency == "" || len(idempotency) > 128 {
		return nil, false, fmt.Errorf("%w: idempotency_key is required and must be at most 128 characters", ErrInvalidRecharge)
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return nil, false, fmt.Errorf("%w: operator is required", ErrInvalidRecharge)
	}
	payloadHash := rechargePayloadHash(snapshot.UserID, amount, request.RaiseKeys)
	now := time.Now().UTC()
	source := strings.TrimSpace(request.Source)
	if source == "" {
		source = "manual"
	}
	record := &RechargeRecord{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now,
		UserID: snapshot.UserID, Email: snapshot.Email,
		Amount: microToUSD(amount), AmountUSDMicro: amount,
		RaiseKeys: request.RaiseKeys, KeysUpdated: "[]",
		Operator: operator, Reason: strings.TrimSpace(request.Reason),
		IdempotencyKey: idempotency, PayloadHash: payloadHash,
		Source: source, SourceRef: strings.TrimSpace(request.SourceRef),
		Status: RechargeApproved, Version: 1,
	}
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx, db, snapshot.UserID, holder)
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, ErrRechargeBusy
	}
	defer releaseUserLease(context.Background(), db, snapshot.UserID, holder)
	created := false
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing RechargeRecord
		existingResult := tx.Where("user_id = ? AND idempotency_key = ?", snapshot.UserID, idempotency).Limit(1).Find(&existing)
		if existingResult.Error != nil {
			return existingResult.Error
		}
		if existingResult.RowsAffected == 1 {
			*record = existing
			return nil
		}
		var pending int64
		if err := tx.Model(&RechargeRecord{}).Where("user_id = ? AND status IN ?", snapshot.UserID, pendingRechargeStatuses()).Count(&pending).Error; err != nil {
			return err
		}
		if pending != 0 {
			return ErrPendingRecharge
		}
		managementPending, err := hasPendingManagement(ctx, tx, snapshot.UserID, snapshot.Email, "")
		if err != nil {
			return err
		}
		if managementPending {
			return ErrManagementPending
		}
		if err := tx.Create(record).Error; err != nil {
			return err
		}
		created = true
		return createAttempt(tx, record, "reserve", payloadHash, "reserved")
	})
	if err != nil {
		return nil, false, err
	}
	if record.PayloadHash != payloadHash {
		return record, false, ErrIdempotencyConflict
	}
	return record, created, nil
}

func pendingRechargeStatuses() []string {
	return []string{RechargeApproved, RechargeExecuting, RechargeAppliedUnverified, RechargeRetryableFailed, RechargeReconcileRequired}
}

func createAttempt(db *gorm.DB, record *RechargeRecord, step, digest, result string) error {
	var count int64
	if err := db.Model(&OperationAttempt{}).Where("operation_type = ? AND operation_id = ? AND step = ?", "recharge", record.ID, step).Count(&count).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	operator := record.Operator
	if strings.TrimSpace(record.AuditOperator) != "" {
		operator = strings.TrimSpace(record.AuditOperator)
	}
	return db.Create(&OperationAttempt{
		ID: newSnapshotID(), OperationType: "recharge", OperationID: record.ID,
		Step: step, Attempt: int(count + 1), RequestDigest: digest,
		ResultCode: result, StartedAt: now, FinishedAt: &now, Operator: operator,
	}).Error
}

func acquireUserLease(ctx context.Context, db *gorm.DB, userID, holder string) (bool, error) {
	now := time.Now().UTC()
	lease := UserLease{UserID: userID, Holder: holder, LeaseUntil: now.Add(rechargeLeaseDuration), UpdatedAt: now}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true,
	}).Create(&lease).Error; err != nil {
		return false, err
	}
	result := db.WithContext(ctx).Model(&UserLease{}).
		Where("user_id = ? AND (holder = ? OR lease_until <= ?)", userID, holder, now).
		Updates(map[string]any{"holder": holder, "lease_until": lease.LeaseUntil, "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func releaseUserLease(ctx context.Context, db *gorm.DB, userID, holder string) {
	now := time.Now().UTC()
	_ = db.WithContext(ctx).Model(&UserLease{}).
		Where("user_id = ? AND holder = ?", userID, holder).
		Updates(map[string]any{"holder": "", "lease_until": now, "updated_at": now}).Error
}

func renewUserLease(ctx context.Context, db *gorm.DB, userID, holder string) error {
	now := time.Now().UTC()
	result := db.WithContext(ctx).Model(&UserLease{}).
		Where("user_id = ? AND holder = ? AND lease_until > ?", userID, holder, now).
		Updates(map[string]any{"lease_until": now.Add(rechargeLeaseDuration), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRechargeBusy
	}
	return nil
}

type rechargeLeaseGuardKey struct{}

// withRechargeLeaseGuard lets a caller fence a higher-level workflow (for
// example a sales order) at every recharge phase without coupling the durable
// recharge command to that workflow's schema.
func withRechargeLeaseGuard(ctx context.Context, guard func(context.Context) error) context.Context {
	return context.WithValue(ctx, rechargeLeaseGuardKey{}, guard)
}

func renewRechargeLeases(ctx context.Context, db *gorm.DB, userID, holder string) error {
	if err := renewUserLease(ctx, db, userID, holder); err != nil {
		return err
	}
	if guard, ok := ctx.Value(rechargeLeaseGuardKey{}).(func(context.Context) error); ok && guard != nil {
		return guard(ctx)
	}
	return nil
}

func confirmUserTarget(ctx context.Context, db *gorm.DB, client *Client, userID, holder string, target int64) (bool, error) {
	var lastErr error
	for _, delay := range rechargeConfirmDelays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
			}
		}
		if err := renewRechargeLeases(ctx, db, userID, holder); err != nil {
			return false, err
		}
		user, err := client.GetUser(ctx, userID)
		if err != nil {
			lastErr = err
			continue
		}
		if user.MaxBudget == nil {
			lastErr = ErrUnlimitedBudget
			continue
		}
		value, err := usdToMicro(*user.MaxBudget)
		if err != nil {
			lastErr = err
			continue
		}
		if value == target {
			return true, nil
		}
		lastErr = nil
	}
	return false, lastErr
}

func confirmKeyTarget(ctx context.Context, db *gorm.DB, client *Client, userID, holder, prefix string, target int64) (bool, error) {
	var lastErr error
	for _, delay := range rechargeConfirmDelays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
			}
		}
		if err := renewRechargeLeases(ctx, db, userID, holder); err != nil {
			return false, err
		}
		keys, err := client.ListKeys(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		for _, key := range keys {
			if key.UserID != userID || key.Prefix() != prefix || key.MaxBudget == nil {
				continue
			}
			value, conversionErr := usdToMicro(*key.MaxBudget)
			if conversionErr == nil && value >= target {
				return true, nil
			}
		}
		lastErr = nil
	}
	return false, lastErr
}

func saveRecharge(ctx context.Context, db *gorm.DB, record *RechargeRecord, values map[string]any) error {
	values["updated_at"] = time.Now().UTC()
	values["version"] = gorm.Expr("version + 1")
	result := db.WithContext(ctx).Model(&RechargeRecord{}).
		Where("id = ? AND version = ?", record.ID, record.Version).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRechargeBusy
	}
	auditOperator := record.AuditOperator
	if err := db.WithContext(ctx).First(record, "id = ?", record.ID).Error; err != nil {
		return err
	}
	record.AuditOperator = auditOperator
	return nil
}

func failRecharge(ctx context.Context, db *gorm.DB, record *RechargeRecord, status, code, step string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		values := map[string]any{"status": status, "last_error_code": code}
		if status == RechargeAppliedUnverified && record.UncertainSince == nil {
			now := time.Now().UTC()
			values["uncertain_since"] = now
		}
		if err := saveRecharge(ctx, tx, record, values); err != nil {
			return err
		}
		return createAttempt(tx, record, step, record.PayloadHash, code)
	})
}

func completeRecharge(ctx context.Context, db *gorm.DB, record *RechargeRecord, keys []string, step string) error {
	encodedKeys, _ := json.Marshal(keys)
	completedAt := time.Now().UTC()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := saveRecharge(ctx, tx, record, map[string]any{
			"status": RechargeCompleted, "last_error_code": "", "keys_updated": string(encodedKeys), "completed_at": completedAt,
			"uncertain_since": nil, "last_observed_usd_micro": nil, "last_observed_at": nil,
			"last_observed_key_prefix": "", "last_observed_key_usd_micro": nil, "last_observed_key_at": nil,
		}); err != nil {
			return err
		}
		return createAttempt(tx, record, step, record.PayloadHash, "completed")
	})
}

func observeRechargeBudget(ctx context.Context, db *gorm.DB, record *RechargeRecord, value int64) (bool, error) {
	now := time.Now().UTC()
	sameValue := record.LastObservedUSDMicro != nil && *record.LastObservedUSDMicro == value
	stable := sameValue &&
		record.LastObservedAt != nil && now.Sub(*record.LastObservedAt) >= rechargeStableObservationWindow
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		values := map[string]any{}
		// Preserve the first observation time while the value remains stable;
		// frequent operator reconciliation must not postpone eligibility forever.
		if !sameValue || record.LastObservedAt == nil {
			values["last_observed_usd_micro"] = value
			values["last_observed_at"] = now
		}
		if err := saveRecharge(ctx, tx, record, values); err != nil {
			return err
		}
		return createAttempt(tx, record, "observe_user_budget", record.PayloadHash, fmt.Sprintf("observed:%d", value))
	})
	return stable, err
}

func uncertainRetryWindowElapsed(record *RechargeRecord, now time.Time) bool {
	return record.UncertainSince != nil && !record.UncertainSince.After(now) &&
		now.Sub(*record.UncertainSince) >= rechargeUncertainRetryAfter
}

func observeKeyBudget(ctx context.Context, db *gorm.DB, record *RechargeRecord, prefix string, value int64) (bool, error) {
	now := time.Now().UTC()
	same := record.LastObservedKeyPrefix == prefix && record.LastObservedKeyUSDMicro != nil &&
		*record.LastObservedKeyUSDMicro == value && record.LastObservedKeyAt != nil
	stable := same && now.Sub(*record.LastObservedKeyAt) >= rechargeStableObservationWindow
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		values := map[string]any{}
		if !same {
			values["last_observed_key_prefix"] = prefix
			values["last_observed_key_usd_micro"] = value
			values["last_observed_key_at"] = now
		}
		if err := saveRecharge(ctx, tx, record, values); err != nil {
			return err
		}
		return createAttempt(tx, record, "observe_key:"+prefix, rechargePayloadHash(prefix, value, false), fmt.Sprintf("observed:%d", value))
	})
	return stable, err
}

// ApplyRecharge first reserves the command and then executes its persisted
// absolute target. It is safe to call again with the same idempotency key.
func ApplyRecharge(ctx context.Context, db *gorm.DB, client *Client, snapshot UserSnapshot, request RechargeRequest, operator string) (*RechargeRecord, error) {
	if db == nil || client == nil {
		return nil, errors.New("litellmops recharge requires a database and a LiteLLM client")
	}
	record, _, err := reserveRecharge(ctx, db, snapshot, request, operator)
	if err != nil {
		return record, err
	}
	if record.Status == RechargeCompleted {
		return record, nil
	}
	var head RechargeRecord
	headResult := db.WithContext(ctx).Where("user_id = ? AND status IN ?", record.UserID, pendingRechargeStatuses()).Order("created_at ASC, id ASC").Limit(1).Find(&head)
	if headResult.Error != nil {
		return record, headResult.Error
	}
	if headResult.RowsAffected == 1 && head.ID != record.ID {
		return record, ErrPendingRecharge
	}
	if record.Status == RechargeTerminalFailed {
		return record, ErrReconcileRequired
	}
	if record.TargetAfterUSDMicro != nil && (record.Status == RechargeExecuting || record.Status == RechargeAppliedUnverified || record.Status == RechargeReconcileRequired) {
		return reconcileRechargeReadOnly(ctx, db, client, record)
	}
	return executeRecharge(ctx, db, client, record)
}

func ReconcileRecharge(ctx context.Context, db *gorm.DB, client *Client, rechargeID string) (*RechargeRecord, error) {
	return ReconcileRechargeAs(ctx, db, client, rechargeID, "")
}

// ReconcileRechargeAs records the operator who performed this reconciliation
// without changing the immutable creator on the recharge command itself.
func ReconcileRechargeAs(ctx context.Context, db *gorm.DB, client *Client, rechargeID, actor string) (*RechargeRecord, error) {
	var record RechargeRecord
	if err := db.WithContext(ctx).First(&record, "id = ?", rechargeID).Error; err != nil {
		return nil, err
	}
	record.AuditOperator = strings.TrimSpace(actor)
	if record.Status == RechargeCompleted {
		return &record, nil
	}
	if record.Status == RechargeTerminalFailed {
		return &record, ErrReconcileRequired
	}
	if record.TargetAfterUSDMicro != nil && (record.Status == RechargeExecuting || record.Status == RechargeAppliedUnverified || record.Status == RechargeReconcileRequired) {
		return reconcileRechargeReadOnly(ctx, db, client, &record)
	}
	return executeRecharge(ctx, db, client, &record)
}

// reconcileRechargeReadOnly first converges exclusively by authoritative
// reads. Only after a durable uncertainty window and two stable observations
// of the exact pre-write value may it re-send the same absolute target. A
// different command remains fenced until this one is resolved.
func reconcileRechargeReadOnly(ctx context.Context, db *gorm.DB, client *Client, record *RechargeRecord) (*RechargeRecord, error) {
	if record.TargetAfterUSDMicro == nil {
		return record, ErrReconcileRequired
	}
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx, db, record.UserID, holder)
	if err != nil {
		return record, err
	}
	if !acquired {
		return record, ErrRechargeBusy
	}
	defer releaseUserLease(context.Background(), db, record.UserID, holder)
	auditOperator := record.AuditOperator
	if err := db.WithContext(ctx).First(record, "id = ?", record.ID).Error; err != nil {
		return record, err
	}
	record.AuditOperator = auditOperator
	var head RechargeRecord
	result := db.WithContext(ctx).Where("user_id = ? AND status IN ?", record.UserID, pendingRechargeStatuses()).Order("created_at ASC, id ASC").Limit(1).Find(&head)
	if result.Error != nil {
		return record, result.Error
	}
	if result.RowsAffected == 1 && head.ID != record.ID {
		return record, ErrPendingRecharge
	}
	target := *record.TargetAfterUSDMicro
	confirmed, readErr := confirmUserTarget(ctx, db, client, record.UserID, holder, target)
	if !confirmed {
		live, finalErr := client.GetUser(ctx, record.UserID)
		if finalErr != nil {
			if readErr == nil {
				readErr = finalErr
			}
			if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "user_readback_failed", "reconcile_read_user"); stateErr != nil {
				return record, stateErr
			}
			return record, ErrReconcileRequired
		}
		status, code := RechargeAppliedUnverified, "target_not_visible"
		if live.MaxBudget == nil {
			status, code = RechargeReconcileRequired, "live_budget_unlimited"
		} else if liveMicro, conversionErr := usdToMicro(*live.MaxBudget); conversionErr != nil {
			status, code = RechargeReconcileRequired, "invalid_live_budget"
		} else if record.BeforeBudgetUSDMicro == nil || liveMicro != *record.BeforeBudgetUSDMicro {
			status, code = RechargeReconcileRequired, "live_budget_changed"
		} else {
			stable, observeErr := observeRechargeBudget(ctx, db, record, liveMicro)
			if observeErr != nil {
				return record, observeErr
			}
			if stable && uncertainRetryWindowElapsed(record, time.Now().UTC()) {
				if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
					return record, err
				}
				if err := createAttempt(db.WithContext(ctx), record, "rewrite_user_started", record.PayloadHash, "started"); err != nil {
					return record, err
				}
				writeErr := client.UpdateUserBudget(ctx, record.UserID, microToUSD(target))
				confirmed, _ := confirmUserTarget(ctx, db, client, record.UserID, holder, target)
				if confirmed {
					if err := createAttempt(db.WithContext(ctx), record, "rewrite_user", record.PayloadHash, "confirmed"); err != nil {
						return record, err
					}
				} else {
					if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "user_rewrite_unverified", "rewrite_user"); stateErr != nil {
						return record, stateErr
					}
					if writeErr != nil && !upstreamResultUncertain(writeErr) {
						return record, writeErr
					}
					return record, ErrReconcileRequired
				}
			} else {
				if stateErr := failRecharge(ctx, db, record, status, code, "reconcile_read_user"); stateErr != nil {
					return record, stateErr
				}
				return record, ErrReconcileRequired
			}
		}
		if status != RechargeAppliedUnverified || code != "target_not_visible" {
			if stateErr := failRecharge(ctx, db, record, status, code, "reconcile_read_user"); stateErr != nil {
				return record, stateErr
			}
			return record, ErrReconcileRequired
		}
	}
	if record.RaiseKeys {
		if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
			return record, err
		}
		keys, listErr := client.ListKeys(ctx)
		if listErr != nil {
			if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "key_readback_failed", "reconcile_read_keys"); stateErr != nil {
				return record, stateErr
			}
			return record, ErrReconcileRequired
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].Prefix() < keys[j].Prefix() })
		keyTarget := target
		if keyTarget > KeyBudgetCeilingMicro {
			keyTarget = KeyBudgetCeilingMicro
		}
		for _, key := range keys {
			if key.UserID != record.UserID || key.IsSession() || key.MaxBudget == nil {
				continue
			}
			current, conversionErr := usdToMicro(*key.MaxBudget)
			if conversionErr != nil {
				if stateErr := failRecharge(ctx, db, record, RechargeReconcileRequired, "invalid_key_budget", "reconcile_read_keys"); stateErr != nil {
					return record, stateErr
				}
				return record, ErrReconcileRequired
			}
			if current < keyTarget {
				stable, observeErr := observeKeyBudget(ctx, db, record, key.Prefix(), current)
				if observeErr != nil {
					return record, observeErr
				}
				if stable && uncertainRetryWindowElapsed(record, time.Now().UTC()) {
					if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
						return record, err
					}
					digest := rechargePayloadHash(key.Prefix(), keyTarget, false)
					if err := createAttempt(db.WithContext(ctx), record, "rewrite_key_started:"+key.Prefix(), digest, "started"); err != nil {
						return record, err
					}
					writeErr := client.UpdateKeyBudget(ctx, key.TokenHash, microToUSD(keyTarget))
					confirmed, _ := confirmKeyTarget(ctx, db, client, record.UserID, holder, key.Prefix(), keyTarget)
					if confirmed {
						if err := createAttempt(db.WithContext(ctx), record, "rewrite_key:"+key.Prefix(), digest, "confirmed"); err != nil {
							return record, err
						}
						continue
					}
					if writeErr != nil && !upstreamResultUncertain(writeErr) {
						if stateErr := failRecharge(ctx, db, record, RechargeReconcileRequired, "key_rewrite_failed", "rewrite_key:"+key.Prefix()); stateErr != nil {
							return record, stateErr
						}
						return record, writeErr
					}
				}
				if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "key_target_not_visible", "reconcile_read_keys"); stateErr != nil {
					return record, stateErr
				}
				return record, ErrReconcileRequired
			}
		}
	}
	var updated []string
	_ = json.Unmarshal([]byte(record.KeysUpdated), &updated)
	if err := completeRecharge(ctx, db, record, updated, "reconcile_complete"); err != nil {
		return record, err
	}
	if _, syncErr := SyncSnapshots(ctx, db, client); syncErr != nil {
		_ = createAttempt(db.WithContext(ctx), record, "sync", record.PayloadHash, "sync_failed")
	}
	return record, nil
}

func executeRecharge(ctx context.Context, db *gorm.DB, client *Client, record *RechargeRecord) (*RechargeRecord, error) {
	if record.TargetAfterUSDMicro != nil && (record.Status == RechargeExecuting || record.Status == RechargeAppliedUnverified || record.Status == RechargeReconcileRequired) {
		return reconcileRechargeReadOnly(ctx, db, client, record)
	}
	holder := newSnapshotID()
	acquired, err := acquireUserLease(ctx, db, record.UserID, holder)
	if err != nil {
		return record, err
	}
	if !acquired {
		return record, ErrRechargeBusy
	}
	defer releaseUserLease(context.Background(), db, record.UserID, holder)
	auditOperator := record.AuditOperator
	if err := db.WithContext(ctx).First(record, "id = ?", record.ID).Error; err != nil {
		return record, err
	}
	record.AuditOperator = auditOperator
	if record.Status == RechargeCompleted {
		return record, nil
	}
	if record.Source == "legacy" || (record.Status != RechargeApproved && record.Status != RechargeRetryableFailed) {
		return record, ErrReconcileRequired
	}
	var head RechargeRecord
	headResult := db.WithContext(ctx).Where("user_id = ? AND status IN ?", record.UserID, pendingRechargeStatuses()).Order("created_at ASC, id ASC").Limit(1).Find(&head)
	if headResult.Error != nil {
		return record, headResult.Error
	}
	if headResult.RowsAffected == 1 && head.ID != record.ID {
		return record, ErrPendingRecharge
	}
	if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
		return record, err
	}
	live, err := client.GetUser(ctx, record.UserID)
	if err != nil {
		if stateErr := failRecharge(ctx, db, record, RechargeRetryableFailed, "user_read_failed", "read_user"); stateErr != nil {
			return record, stateErr
		}
		return record, err
	}
	if live.MaxBudget == nil {
		if stateErr := failRecharge(ctx, db, record, RechargeTerminalFailed, "unlimited_budget", "read_user"); stateErr != nil {
			return record, stateErr
		}
		return record, ErrUnlimitedBudget
	}
	liveMicro, err := usdToMicro(*live.MaxBudget)
	if err != nil {
		if stateErr := failRecharge(ctx, db, record, RechargeTerminalFailed, "invalid_upstream_budget", "read_user"); stateErr != nil {
			return record, stateErr
		}
		return record, ErrReconcileRequired
	}
	if record.TargetAfterUSDMicro == nil {
		if liveMicro > math.MaxInt64-record.AmountUSDMicro {
			if stateErr := failRecharge(ctx, db, record, RechargeTerminalFailed, "budget_overflow", "target"); stateErr != nil {
				return record, stateErr
			}
			return record, ErrInvalidRecharge
		}
		target := liveMicro + record.AmountUSDMicro
		if err := saveRecharge(ctx, db, record, map[string]any{
			"before_budget_usd_micro": liveMicro, "target_after_usd_micro": target,
			"before_budget": microToUSD(liveMicro), "after_budget": microToUSD(target),
			"status": RechargeExecuting, "last_error_code": "",
		}); err != nil {
			return record, err
		}
	} else if record.Status != RechargeExecuting {
		if err := saveRecharge(ctx, db, record, map[string]any{"status": RechargeExecuting, "last_error_code": ""}); err != nil {
			return record, err
		}
	}
	target := *record.TargetAfterUSDMicro
	if liveMicro > target {
		if stateErr := failRecharge(ctx, db, record, RechargeReconcileRequired, "live_budget_above_target", "write_user"); stateErr != nil {
			return record, stateErr
		}
		return record, ErrReconcileRequired
	}
	if liveMicro < target {
		if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
			return record, err
		}
		if err := createAttempt(db.WithContext(ctx), record, "write_user_started", record.PayloadHash, "started"); err != nil {
			return record, err
		}
		writeErr := client.UpdateUserBudget(ctx, record.UserID, microToUSD(target))
		if writeErr != nil && upstreamResultUncertain(writeErr) {
			confirmed, _ := confirmUserTarget(ctx, db, client, record.UserID, holder, target)
			if confirmed {
				writeErr = nil
			}
		}
		// A 2xx proves only that LiteLLM accepted the request. Confirm the
		// source-of-truth value before advancing to key updates/completion.
		if writeErr == nil {
			confirmed, _ := confirmUserTarget(ctx, db, client, record.UserID, holder, target)
			if !confirmed {
				if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "user_write_unverified", "confirm_user"); stateErr != nil {
					return record, stateErr
				}
				return record, ErrReconcileRequired
			}
		}
		if writeErr != nil {
			status, code := RechargeRetryableFailed, "user_write_failed"
			if upstreamResultUncertain(writeErr) {
				status, code = RechargeAppliedUnverified, "user_write_unverified"
			}
			var upstream *UpstreamError
			if errors.As(writeErr, &upstream) && upstream.StatusCode >= http.StatusBadRequest && upstream.StatusCode < http.StatusInternalServerError {
				status = RechargeTerminalFailed
			}
			if stateErr := failRecharge(ctx, db, record, status, code, "write_user"); stateErr != nil {
				return record, stateErr
			}
			return record, writeErr
		}
		if err := createAttempt(db.WithContext(ctx), record, "write_user", record.PayloadHash, "confirmed"); err != nil {
			return record, err
		}
	}

	updatedPrefixes := make([]string, 0)
	if record.RaiseKeys {
		if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
			return record, err
		}
		keys, listErr := client.ListKeys(ctx)
		if listErr != nil {
			if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "key_list_failed", "list_keys"); stateErr != nil {
				return record, stateErr
			}
			return record, ErrReconcileRequired
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].Prefix() < keys[j].Prefix() })
		for _, key := range keys {
			if key.UserID != record.UserID || key.IsSession() || key.MaxBudget == nil {
				continue
			}
			current, conversionErr := usdToMicro(*key.MaxBudget)
			if conversionErr != nil {
				continue
			}
			keyTarget := target
			if keyTarget > KeyBudgetCeilingMicro {
				keyTarget = KeyBudgetCeilingMicro
			}
			if current >= keyTarget {
				continue
			}
			if err := renewRechargeLeases(ctx, db, record.UserID, holder); err != nil {
				return record, err
			}
			digest := rechargePayloadHash(key.Prefix(), keyTarget, false)
			if err := createAttempt(db.WithContext(ctx), record, "write_key_started:"+key.Prefix(), digest, "started"); err != nil {
				return record, err
			}
			keyErr := client.UpdateKeyBudget(ctx, key.TokenHash, microToUSD(keyTarget))
			confirmed, _ := confirmKeyTarget(ctx, db, client, record.UserID, holder, key.Prefix(), keyTarget)
			if !confirmed {
				if stateErr := failRecharge(ctx, db, record, RechargeAppliedUnverified, "key_write_unverified", "write_key:"+key.Prefix()); stateErr != nil {
					return record, stateErr
				}
				return record, ErrReconcileRequired
			}
			_ = keyErr // readback is authoritative even when the response was lost.
			updatedPrefixes = append(updatedPrefixes, key.Prefix())
			if err := createAttempt(db.WithContext(ctx), record, "write_key:"+key.Prefix(), digest, "confirmed"); err != nil {
				return record, err
			}
		}
	}
	if err := completeRecharge(ctx, db, record, updatedPrefixes, "complete"); err != nil {
		return record, err
	}
	if _, syncErr := SyncSnapshots(ctx, db, client); syncErr != nil {
		_ = createAttempt(db.WithContext(ctx), record, "sync", record.PayloadHash, "sync_failed")
	}
	return record, nil
}
