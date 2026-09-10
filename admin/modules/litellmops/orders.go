package litellmops

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const envConnectorSharedToken = "LITELLMOPS_CONNECTOR_SHARED_TOKEN"
const envConnectorAllowedSources = "LITELLMOPS_CONNECTOR_ALLOWED_SOURCES"
const orderExecutionLeaseDuration = 10 * time.Minute

const (
	OrderReceived          = "received"
	OrderVerifiedPaid      = "verified_paid"
	OrderMapped            = "mapped"
	OrderApproved          = "approved"
	OrderExecuting         = "executing"
	OrderAppliedUnverified = "applied_unverified"
	OrderCompleted         = "completed"
	OrderRetryableFailed   = "retryable_failed"
	OrderTerminalFailed    = "terminal_failed"
	OrderReconcileRequired = "reconcile_required"
	OrderRefundReview      = "refund_review"
	OrderReversed          = "reversed"
)

var (
	ErrInvalidProduct    = errors.New("litellmops invalid sales product")
	ErrInvalidOrder      = errors.New("litellmops invalid sales order")
	ErrOrderConflict     = errors.New("litellmops sales order conflict")
	ErrInvalidOrderState = errors.New("litellmops invalid sales order state")
	ErrOrderBusy         = errors.New("litellmops sales order is already executing")
)

type SalesProduct struct {
	ID             string    `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null" json:"updated_at"`
	Channel        string    `gorm:"column:channel;type:varchar(32);not null;uniqueIndex:ux_litellmops_product_mapping,priority:1" json:"channel"`
	Shop           string    `gorm:"column:shop;type:varchar(128);not null;uniqueIndex:ux_litellmops_product_mapping,priority:2" json:"shop"`
	ExternalItemID string    `gorm:"column:external_item_id;type:varchar(128);not null;uniqueIndex:ux_litellmops_product_mapping,priority:3" json:"external_item_id"`
	SKU            string    `gorm:"column:sku;type:varchar(128);not null;default:'';uniqueIndex:ux_litellmops_product_mapping,priority:4" json:"sku"`
	Title          string    `gorm:"column:title;type:varchar(254);not null" json:"title"`
	PriceCNYFen    int64     `gorm:"column:price_cny_fen;not null" json:"price_cny_fen"`
	CreditUSDMicro int64     `gorm:"column:credit_usd_micro;not null" json:"credit_usd_micro"`
	RaiseKeys      bool      `gorm:"column:raise_keys;not null;default:true" json:"raise_keys"`
	Enabled        bool      `gorm:"column:enabled;not null;default:false;index" json:"enabled"`
	AutoApply      bool      `gorm:"column:auto_apply;not null;default:false" json:"auto_apply"`
	Version        int64     `gorm:"column:version;not null;default:1" json:"version"`
}

func (SalesProduct) TableName() string { return "litellmops_sales_product" }

type SalesOrder struct {
	ID                  string     `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt           time.Time  `gorm:"column:created_at;not null;index" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
	CompletedAt         *time.Time `gorm:"column:completed_at" json:"completed_at"`
	Channel             string     `gorm:"column:channel;type:varchar(32);not null;uniqueIndex:ux_litellmops_sales_order,priority:1" json:"channel"`
	Shop                string     `gorm:"column:shop;type:varchar(128);not null;uniqueIndex:ux_litellmops_sales_order,priority:2" json:"shop"`
	ExternalOrderID     string     `gorm:"column:external_order_id;type:varchar(128);not null;uniqueIndex:ux_litellmops_sales_order,priority:3" json:"external_order_id"`
	AdjustmentType      string     `gorm:"column:adjustment_type;type:varchar(32);not null;uniqueIndex:ux_litellmops_sales_order,priority:4" json:"adjustment_type"`
	PayloadHash         string     `gorm:"column:payload_hash;type:varchar(64);not null" json:"payload_hash"`
	ProductID           string     `gorm:"column:product_id;type:varchar(64);index" json:"product_id"`
	ExternalItemID      string     `gorm:"column:external_item_id;type:varchar(128);index" json:"external_item_id"`
	SKU                 string     `gorm:"column:sku;type:varchar(128)" json:"sku"`
	UserID              string     `gorm:"column:user_id;type:varchar(64);index" json:"user_id"`
	UserEmail           string     `gorm:"column:user_email;type:varchar(254);index" json:"user_email"`
	PaidCNYFen          int64      `gorm:"column:paid_cny_fen;not null" json:"paid_cny_fen"`
	CreditUSDMicro      int64      `gorm:"column:credit_usd_micro;not null;default:0" json:"credit_usd_micro"`
	RaiseKeys           bool       `gorm:"column:raise_keys;not null;default:true" json:"raise_keys"`
	PaymentStatus       string     `gorm:"column:payment_status;type:varchar(32);not null" json:"payment_status"`
	SourceTrust         string     `gorm:"column:source_trust;type:varchar(32);not null" json:"source_trust"`
	Status              string     `gorm:"column:status;type:varchar(32);not null;index" json:"status"`
	RechargeID          string     `gorm:"column:recharge_id;type:varchar(64);index" json:"recharge_id"`
	Operator            string     `gorm:"column:operator;type:varchar(128);not null" json:"operator"`
	Approver            string     `gorm:"column:approver;type:varchar(128)" json:"approver"`
	Note                string     `gorm:"column:note;type:varchar(512)" json:"note"`
	LastErrorCode       string     `gorm:"column:last_error_code;type:varchar(64)" json:"last_error_code"`
	Version             int64      `gorm:"column:version;not null;default:1" json:"version"`
	ExecutionHolder     string     `gorm:"column:execution_holder;type:varchar(64)" json:"-"`
	ExecutionFence      int64      `gorm:"column:execution_fence;not null;default:0" json:"execution_fence"`
	ExecutionLeaseUntil *time.Time `gorm:"column:execution_lease_until;index" json:"execution_lease_until"`
}

func (SalesOrder) TableName() string { return "litellmops_sales_order" }

type productCreateRequest struct {
	Channel        string `json:"channel"`
	Shop           string `json:"shop"`
	ExternalItemID string `json:"external_item_id"`
	SKU            string `json:"sku"`
	Title          string `json:"title"`
	PriceCNYFen    int64  `json:"price_cny_fen"`
	CreditUSDMicro int64  `json:"credit_usd_micro"`
	RaiseKeys      *bool  `json:"raise_keys"`
	Enabled        bool   `json:"enabled"`
	AutoApply      bool   `json:"auto_apply"`
}

type productUpdateRequest struct {
	Title          *string `json:"title"`
	PriceCNYFen    *int64  `json:"price_cny_fen"`
	CreditUSDMicro *int64  `json:"credit_usd_micro"`
	RaiseKeys      *bool   `json:"raise_keys"`
	Enabled        *bool   `json:"enabled"`
	AutoApply      *bool   `json:"auto_apply"`
	Version        *int64  `json:"version"`
}

type orderCreateRequest struct {
	Channel         string `json:"channel"`
	Shop            string `json:"shop"`
	ExternalOrderID string `json:"external_order_id"`
	AdjustmentType  string `json:"adjustment_type"`
	ProductID       string `json:"product_id"`
	ExternalItemID  string `json:"external_item_id"`
	SKU             string `json:"sku"`
	UserEmail       string `json:"user_email"`
	PaidCNYFen      int64  `json:"paid_cny_fen"`
	PaymentStatus   string `json:"payment_status"`
	SourceTrust     string `json:"source_trust"`
	Note            string `json:"note"`
}

type orderMatchRequest struct {
	ProductID string `json:"product_id"`
	UserEmail string `json:"user_email"`
}

type orderRefundReviewRequest struct {
	Reason string `json:"reason"`
}

func refundReviewAllowedStatuses() []string {
	return []string{OrderReceived, OrderVerifiedPaid, OrderMapped, OrderApproved, OrderCompleted, OrderTerminalFailed}
}

func normalizeOrderRequest(request orderCreateRequest, trustedImport bool) (orderCreateRequest, error) {
	request.Channel = strings.ToLower(strings.TrimSpace(request.Channel))
	request.Shop = strings.TrimSpace(request.Shop)
	request.ExternalOrderID = strings.TrimSpace(request.ExternalOrderID)
	request.AdjustmentType = strings.ToLower(strings.TrimSpace(request.AdjustmentType))
	request.ProductID = strings.TrimSpace(request.ProductID)
	request.ExternalItemID = strings.TrimSpace(request.ExternalItemID)
	request.SKU = strings.TrimSpace(request.SKU)
	request.UserEmail = strings.ToLower(strings.TrimSpace(request.UserEmail))
	request.PaymentStatus = strings.ToLower(strings.TrimSpace(request.PaymentStatus))
	request.Note = strings.TrimSpace(request.Note)
	if request.AdjustmentType == "" {
		request.AdjustmentType = "credit"
	}
	if request.AdjustmentType == "purchase" {
		request.AdjustmentType = "credit"
	}
	if trustedImport {
		request.SourceTrust = "trusted"
	} else {
		request.SourceTrust = "manual"
	}
	if request.Channel == "" || request.Shop == "" || request.ExternalOrderID == "" || request.PaidCNYFen <= 0 || request.PaymentStatus == "" {
		return request, ErrInvalidOrder
	}
	if request.AdjustmentType != "credit" {
		return request, fmt.Errorf("%w: unsupported adjustment_type", ErrInvalidOrder)
	}
	if request.SourceTrust != "manual" && request.SourceTrust != "trusted" {
		return request, fmt.Errorf("%w: invalid source_trust", ErrInvalidOrder)
	}
	return request, nil
}

func hashOrderRequest(request orderCreateRequest) string {
	raw, _ := json.Marshal(request)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func createSalesOrder(ctx context.Context, db *gorm.DB, request orderCreateRequest, operator string, trustedImport bool) (*SalesOrder, bool, error) {
	request, err := normalizeOrderRequest(request, trustedImport)
	if err != nil {
		return nil, false, err
	}
	operator = strings.TrimSpace(operator)
	if operator == "" {
		return nil, false, fmt.Errorf("%w: operator is required", ErrInvalidOrder)
	}
	now := time.Now().UTC()
	order := &SalesOrder{
		ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now,
		Channel: request.Channel, Shop: request.Shop, ExternalOrderID: request.ExternalOrderID,
		AdjustmentType: request.AdjustmentType, PayloadHash: hashOrderRequest(request),
		ProductID: request.ProductID, ExternalItemID: request.ExternalItemID, SKU: request.SKU,
		UserEmail: request.UserEmail, PaidCNYFen: request.PaidCNYFen,
		PaymentStatus: request.PaymentStatus, SourceTrust: request.SourceTrust,
		Status: OrderReceived, Operator: operator, Note: request.Note, Version: 1,
	}
	created := false
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "channel"}, {Name: "shop"}, {Name: "external_order_id"}, {Name: "adjustment_type"}}, DoNothing: true,
		}).Create(order)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected == 1
		if created {
			return recordOrderAttempt(tx, order, "receive", "received")
		}
		var existing SalesOrder
		if err := tx.Where("channel = ? AND shop = ? AND external_order_id = ? AND adjustment_type = ?",
			request.Channel, request.Shop, request.ExternalOrderID, request.AdjustmentType).First(&existing).Error; err != nil {
			return err
		}
		*order = existing
		return nil
	}); err != nil {
		return nil, false, err
	}
	if order.PayloadHash != hashOrderRequest(request) {
		return order, false, ErrOrderConflict
	}
	return order, created, nil
}

func recordOrderAttempt(db *gorm.DB, order *SalesOrder, step, result string, actors ...string) error {
	var count int64
	if err := db.Model(&OperationAttempt{}).Where("operation_type = ? AND operation_id = ? AND step = ?", "sales_order", order.ID, step).Count(&count).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	operator := order.Operator
	if len(actors) > 0 && strings.TrimSpace(actors[0]) != "" {
		operator = strings.TrimSpace(actors[0])
	}
	return db.Create(&OperationAttempt{
		ID: newSnapshotID(), OperationType: "sales_order", OperationID: order.ID,
		Step: step, Attempt: int(count + 1), RequestDigest: order.PayloadHash,
		ResultCode: result, StartedAt: now, FinishedAt: &now, Operator: operator,
	}).Error
}

func updateOrderState(ctx context.Context, db *gorm.DB, order *SalesOrder, allowed []string, values map[string]any) error {
	values["updated_at"] = time.Now().UTC()
	values["version"] = gorm.Expr("version + 1")
	result := db.WithContext(ctx).Model(&SalesOrder{}).Where("id = ? AND version = ? AND status IN ?", order.ID, order.Version, allowed).Updates(values)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInvalidOrderState
	}
	return db.WithContext(ctx).First(order, "id = ?", order.ID).Error
}

func transitionOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, allowed []string, values map[string]any, step, result string, actors ...string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := updateOrderState(ctx, tx, order, allowed, values); err != nil {
			return err
		}
		return recordOrderAttempt(tx, order, step, result, actors...)
	})
}

func containsOrderStatus(status string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == status {
			return true
		}
	}
	return false
}

func claimOrderExecution(ctx context.Context, db *gorm.DB, order *SalesOrder, allowed []string, actor, step string) (string, int64, error) {
	holder := newSnapshotID()
	now := time.Now().UTC()
	fence := int64(0)
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current SalesOrder
		if err := tx.First(&current, "id = ?", order.ID).Error; err != nil {
			return err
		}
		if current.Status == OrderExecuting {
			fresh := current.ExecutionLeaseUntil != nil && current.ExecutionLeaseUntil.After(now)
			if current.ExecutionLeaseUntil == nil && current.UpdatedAt.After(now.Add(-orderExecutionLeaseDuration)) {
				fresh = true
			}
			if fresh {
				return ErrOrderBusy
			}
		}
		if !containsOrderStatus(current.Status, allowed) {
			return ErrInvalidOrderState
		}
		fence = current.ExecutionFence + 1
		leaseUntil := now.Add(orderExecutionLeaseDuration)
		result := tx.Model(&SalesOrder{}).Where("id = ? AND version = ?", current.ID, current.Version).Updates(map[string]any{
			"status": OrderExecuting, "execution_holder": holder, "execution_fence": fence,
			"execution_lease_until": leaseUntil, "updated_at": now, "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrOrderBusy
		}
		if err := tx.First(&current, "id = ?", current.ID).Error; err != nil {
			return err
		}
		*order = current
		return recordOrderAttempt(tx, order, step, "executing", actor)
	})
	return holder, fence, err
}

func updateClaimedOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, holder string, fence int64, values map[string]any, release bool, step, result, actor string) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		values["updated_at"] = now
		values["version"] = gorm.Expr("version + 1")
		if release {
			values["execution_holder"] = ""
			values["execution_lease_until"] = nil
		} else {
			values["execution_lease_until"] = now.Add(orderExecutionLeaseDuration)
		}
		update := tx.Model(&SalesOrder{}).Where(
			"id = ? AND version = ? AND status = ? AND execution_holder = ? AND execution_fence = ? AND execution_lease_until > ?",
			order.ID, order.Version, OrderExecuting, holder, fence, now,
		).Updates(values)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrOrderBusy
		}
		if err := tx.First(order, "id = ?", order.ID).Error; err != nil {
			return err
		}
		return recordOrderAttempt(tx, order, step, result, actor)
	})
}

func renewClaimedOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, holder string, fence int64) error {
	now := time.Now().UTC()
	update := db.WithContext(ctx).Model(&SalesOrder{}).Where(
		"id = ? AND version = ? AND status = ? AND execution_holder = ? AND execution_fence = ? AND execution_lease_until > ?",
		order.ID, order.Version, OrderExecuting, holder, fence, now,
	).Updates(map[string]any{
		"execution_lease_until": now.Add(orderExecutionLeaseDuration),
		"updated_at":            now,
		"version":               gorm.Expr("version + 1"),
	})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrOrderBusy
	}
	return db.WithContext(ctx).First(order, "id = ?", order.ID).Error
}

func verifySalesOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, actor string) error {
	if order.PaymentStatus != "paid" {
		return fmt.Errorf("%w: payment_status is not paid", ErrInvalidOrder)
	}
	return transitionOrder(ctx, db, order, []string{OrderReceived}, map[string]any{"status": OrderVerifiedPaid, "last_error_code": ""}, "verify", "verified_paid", actor)
}

func matchSalesOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, request orderMatchRequest, actor string) error {
	productID := strings.TrimSpace(request.ProductID)
	if productID == "" {
		productID = order.ProductID
	}
	var product SalesProduct
	if productID != "" {
		if err := db.WithContext(ctx).First(&product, "id = ? AND enabled = ?", productID, true).Error; err != nil {
			return err
		}
	} else {
		if order.ExternalItemID == "" {
			return fmt.Errorf("%w: product_id or external_item_id is required", ErrInvalidOrder)
		}
		if err := db.WithContext(ctx).Where("channel = ? AND shop = ? AND external_item_id = ? AND sku = ? AND enabled = ?",
			order.Channel, order.Shop, order.ExternalItemID, order.SKU, true).First(&product).Error; err != nil {
			return err
		}
	}
	if product.Channel != order.Channel || product.Shop != order.Shop || product.PriceCNYFen != order.PaidCNYFen {
		return fmt.Errorf("%w: product mapping or paid amount does not match exactly", ErrInvalidOrder)
	}
	email := strings.ToLower(strings.TrimSpace(request.UserEmail))
	if email == "" {
		email = order.UserEmail
	}
	if email == "" {
		return fmt.Errorf("%w: exact user_email is required", ErrInvalidOrder)
	}
	var users []UserSnapshot
	if err := db.WithContext(ctx).Where("LOWER(email) = ?", email).Limit(2).Find(&users).Error; err != nil {
		return err
	}
	if len(users) != 1 {
		return fmt.Errorf("%w: user_email must resolve to exactly one user", ErrInvalidOrder)
	}
	return transitionOrder(ctx, db, order, []string{OrderVerifiedPaid}, map[string]any{
		"status": OrderMapped, "product_id": product.ID, "external_item_id": product.ExternalItemID,
		"sku": product.SKU, "user_id": users[0].UserID, "user_email": email,
		"credit_usd_micro": product.CreditUSDMicro, "raise_keys": product.RaiseKeys, "last_error_code": "",
	}, "match", "mapped", actor)
}

func approveSalesOrder(ctx context.Context, db *gorm.DB, order *SalesOrder, approver string) error {
	return transitionOrder(ctx, db, order, []string{OrderMapped}, map[string]any{"status": OrderApproved, "approver": approver, "last_error_code": ""}, "approve", "approved", approver)
}

func executeSalesOrder(ctx context.Context, db *gorm.DB, client *Client, order *SalesOrder, actor string) error {
	holder, fence, err := claimOrderExecution(ctx, db, order, []string{OrderApproved, OrderRetryableFailed}, actor, "execute_start")
	if err != nil {
		return err
	}
	var snapshot UserSnapshot
	if err := db.WithContext(ctx).First(&snapshot, "user_id = ?", order.UserID).Error; err != nil {
		if stateErr := updateClaimedOrder(ctx, db, order, holder, fence, map[string]any{"status": OrderTerminalFailed, "last_error_code": "user_not_found"}, true, "execute", "user_not_found", actor); stateErr != nil {
			return stateErr
		}
		return err
	}
	rechargeRequest := RechargeRequest{
		AmountUSDMicro: order.CreditUSDMicro, RaiseKeys: order.RaiseKeys,
		Reason:         "sales order " + order.ExternalOrderID,
		IdempotencyKey: "sales-order:" + order.ID + ":credit-v1", Source: "sales_order", SourceRef: order.ID,
	}
	recharge, _, err := reserveRecharge(ctx, db, snapshot, rechargeRequest, actor)
	if err != nil {
		status, code := OrderTerminalFailed, "recharge_reservation_failed"
		if errors.Is(err, ErrRechargeBusy) || errors.Is(err, ErrPendingRecharge) || errors.Is(err, ErrManagementPending) {
			status, code = OrderRetryableFailed, "recharge_pending"
		}
		if stateErr := updateClaimedOrder(ctx, db, order, holder, fence, map[string]any{"status": status, "last_error_code": code}, true, "execute", code, actor); stateErr != nil {
			return stateErr
		}
		return err
	}
	// Persist the linkage before the first remote write. A crash after this
	// point is recoverable through the order reconciliation endpoint.
	if err := updateClaimedOrder(ctx, db, order, holder, fence, map[string]any{"status": OrderExecuting, "recharge_id": recharge.ID}, false, "link_recharge", "linked", actor); err != nil {
		return err
	}
	rechargeCtx := withRechargeLeaseGuard(ctx, func(guardCtx context.Context) error {
		return renewClaimedOrder(guardCtx, db, order, holder, fence)
	})
	if recharge.Status != RechargeCompleted {
		if recharge.TargetAfterUSDMicro != nil && (recharge.Status == RechargeExecuting || recharge.Status == RechargeAppliedUnverified || recharge.Status == RechargeReconcileRequired) {
			recharge, err = ReconcileRechargeAs(rechargeCtx, db, client, recharge.ID, actor)
		} else {
			recharge, err = executeRecharge(rechargeCtx, db, client, recharge)
		}
	}
	if renewErr := renewClaimedOrder(ctx, db, order, holder, fence); renewErr != nil {
		return renewErr
	}
	values := map[string]any{}
	if recharge != nil {
		values["recharge_id"] = recharge.ID
	}
	if err == nil && recharge != nil && recharge.Status == RechargeCompleted {
		now := time.Now().UTC()
		values["status"], values["completed_at"], values["last_error_code"] = OrderCompleted, now, ""
		return updateClaimedOrder(ctx, db, order, holder, fence, values, true, "execute", "completed", actor)
	}
	status, code := OrderRetryableFailed, "recharge_failed"
	if recharge != nil {
		switch recharge.Status {
		case RechargeAppliedUnverified:
			status, code = OrderAppliedUnverified, "recharge_unverified"
		case RechargeReconcileRequired:
			status, code = OrderReconcileRequired, "recharge_reconcile_required"
		case RechargeTerminalFailed:
			status, code = OrderTerminalFailed, recharge.LastErrorCode
		}
	}
	values["status"], values["last_error_code"] = status, code
	if stateErr := updateClaimedOrder(ctx, db, order, holder, fence, values, true, "execute", code, actor); stateErr != nil {
		return stateErr
	}
	return err
}

func reconcileSalesOrder(ctx context.Context, db *gorm.DB, client *Client, order *SalesOrder, actor string) error {
	rechargeID := order.RechargeID
	if order.RechargeID == "" {
		var recharge RechargeRecord
		lookupErr := db.WithContext(ctx).Where("source = ? AND source_ref = ?", "sales_order", order.ID).First(&recharge).Error
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}
		if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			if order.Status == OrderRetryableFailed {
				// No remote write was started because reservation was blocked.
				// Reuse the normal execution path and its stable order idempotency key.
				return executeSalesOrder(ctx, db, client, order, actor)
			}
			holder, fence, claimErr := claimOrderExecution(ctx, db, order, []string{OrderExecuting}, actor, "reconcile_start")
			if claimErr != nil {
				return claimErr
			}
			if stateErr := updateClaimedOrder(ctx, db, order, holder, fence,
				map[string]any{"status": OrderRetryableFailed, "last_error_code": "missing_recharge_reservation"},
				true, "reconcile", "missing_recharge_reservation", actor); stateErr != nil {
				return stateErr
			}
			return executeSalesOrder(ctx, db, client, order, actor)
		}
		rechargeID = recharge.ID
	}
	// Claim the order before any remote reconciliation. This excludes a racing
	// refund-review transition and gives every caller a CAS/fencing point.
	holder, fence, err := claimOrderExecution(ctx, db, order,
		[]string{OrderExecuting, OrderAppliedUnverified, OrderRetryableFailed, OrderReconcileRequired}, actor, "reconcile_start")
	if err != nil {
		return err
	}
	if err := updateClaimedOrder(ctx, db, order, holder, fence, map[string]any{"status": OrderExecuting, "recharge_id": rechargeID}, false, "link_recharge", "linked", actor); err != nil {
		return err
	}
	rechargeCtx := withRechargeLeaseGuard(ctx, func(guardCtx context.Context) error {
		return renewClaimedOrder(guardCtx, db, order, holder, fence)
	})
	recharge, err := ReconcileRechargeAs(rechargeCtx, db, client, rechargeID, actor)
	if renewErr := renewClaimedOrder(ctx, db, order, holder, fence); renewErr != nil {
		return renewErr
	}
	if err == nil && recharge.Status == RechargeCompleted {
		now := time.Now().UTC()
		return updateClaimedOrder(ctx, db, order, holder, fence,
			map[string]any{"status": OrderCompleted, "completed_at": now, "last_error_code": ""}, true, "reconcile", "completed", actor)
	}
	status, code := OrderReconcileRequired, "still_unverified"
	if recharge != nil {
		switch recharge.Status {
		case RechargeRetryableFailed:
			status, code = OrderRetryableFailed, "recharge_retryable"
		case RechargeTerminalFailed:
			status, code = OrderTerminalFailed, recharge.LastErrorCode
		case RechargeAppliedUnverified:
			status, code = OrderAppliedUnverified, "recharge_unverified"
		}
	}
	if stateErr := updateClaimedOrder(ctx, db, order, holder, fence,
		map[string]any{"status": status, "last_error_code": code}, true, "reconcile", code, actor); stateErr != nil {
		return stateErr
	}
	return err
}

func loadSalesOrder(ctx context.Context, db *gorm.DB, id string) (*SalesOrder, error) {
	var order SalesOrder
	if err := db.WithContext(ctx).First(&order, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (handler *requestHandler) operator(ctx *gin.Context) string {
	if principal := handler.runtime.Principal(ctx); principal != nil {
		if value := strings.TrimSpace(principal.GetUsername()); value != "" {
			return value
		}
		return strings.TrimSpace(principal.GetUserID())
	}
	return ""
}

func (handler *requestHandler) listProducts(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&SalesProduct{})
	if channel := strings.TrimSpace(ctx.Query("channel")); channel != "" {
		query = query.Where("channel = ?", channel)
	}
	if shop := strings.TrimSpace(ctx.Query("shop")); shop != "" {
		query = query.Where("shop = ?", shop)
	}
	if enabled := strings.TrimSpace(ctx.Query("enabled")); enabled != "" {
		query = query.Where("enabled = ?", enabled == "true" || enabled == "1")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	page := bindPage(ctx)
	items := make([]SalesProduct, 0, page.PageSize)
	if err := query.Order("created_at DESC").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&items).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) createProduct(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var request productCreateRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidProduct)
		return
	}
	request.Channel = strings.ToLower(strings.TrimSpace(request.Channel))
	request.Shop = strings.TrimSpace(request.Shop)
	request.ExternalItemID = strings.TrimSpace(request.ExternalItemID)
	request.SKU = strings.TrimSpace(request.SKU)
	request.Title = strings.TrimSpace(request.Title)
	if request.Channel == "" || request.Shop == "" || request.ExternalItemID == "" || request.Title == "" || request.PriceCNYFen <= 0 || request.CreditUSDMicro < minRechargeMicro || request.CreditUSDMicro > maxRechargeMicro || (request.AutoApply && !request.Enabled) {
		writeAPIError(ctx, ErrInvalidProduct)
		return
	}
	raise := true
	if request.RaiseKeys != nil {
		raise = *request.RaiseKeys
	}
	now := time.Now().UTC()
	product := SalesProduct{ID: newSnapshotID(), CreatedAt: now, UpdatedAt: now, Channel: request.Channel, Shop: request.Shop, ExternalItemID: request.ExternalItemID, SKU: request.SKU, Title: request.Title, PriceCNYFen: request.PriceCNYFen, CreditUSDMicro: request.CreditUSDMicro, RaiseKeys: raise, Enabled: request.Enabled, AutoApply: request.AutoApply, Version: 1}
	if err := db.Create(&product).Error; err != nil {
		writeAPIError(ctx, ErrOrderConflict)
		return
	}
	ctx.JSON(201, product)
}

func (handler *requestHandler) updateProduct(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	var product SalesProduct
	if err := db.First(&product, "id = ?", ctx.Param("id")).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	var request productUpdateRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidProduct)
		return
	}
	if request.Version == nil || *request.Version < 1 {
		writeAPIError(ctx, fmt.Errorf("%w: version is required", ErrInvalidProduct))
		return
	}
	values := map[string]any{"updated_at": time.Now().UTC(), "version": gorm.Expr("version + 1")}
	if request.Title != nil {
		title := strings.TrimSpace(*request.Title)
		if title == "" {
			writeAPIError(ctx, ErrInvalidProduct)
			return
		}
		values["title"] = title
	}
	if request.PriceCNYFen != nil {
		if *request.PriceCNYFen <= 0 {
			writeAPIError(ctx, ErrInvalidProduct)
			return
		}
		values["price_cny_fen"] = *request.PriceCNYFen
	}
	if request.CreditUSDMicro != nil {
		if *request.CreditUSDMicro < minRechargeMicro || *request.CreditUSDMicro > maxRechargeMicro {
			writeAPIError(ctx, ErrInvalidProduct)
			return
		}
		values["credit_usd_micro"] = *request.CreditUSDMicro
	}
	if request.RaiseKeys != nil {
		values["raise_keys"] = *request.RaiseKeys
	}
	if request.Enabled != nil {
		values["enabled"] = *request.Enabled
	}
	if request.AutoApply != nil {
		values["auto_apply"] = *request.AutoApply
	}
	enabled, auto := product.Enabled, product.AutoApply
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	if request.AutoApply != nil {
		auto = *request.AutoApply
	}
	if auto && !enabled {
		writeAPIError(ctx, ErrInvalidProduct)
		return
	}
	result := db.Model(&SalesProduct{}).Where("id = ? AND version = ?", product.ID, *request.Version).Updates(values)
	if result.Error != nil {
		writeAPIError(ctx, result.Error)
		return
	}
	if result.RowsAffected != 1 {
		writeAPIError(ctx, ErrOrderConflict)
		return
	}
	_ = db.First(&product, "id = ?", product.ID).Error
	ctx.JSON(200, product)
}

func (handler *requestHandler) listOrders(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	query := db.Model(&SalesOrder{})
	for key, column := range map[string]string{
		"channel": "channel", "shop": "shop", "status": "status", "external_order_id": "external_order_id",
		"user_email": "user_email", "payment_status": "payment_status", "source_trust": "source_trust",
		"adjustment_type": "adjustment_type", "product_id": "product_id",
	} {
		if value := strings.TrimSpace(ctx.Query(key)); value != "" {
			query = query.Where(column+" = ?", value)
		}
	}
	if since := parseTimeQuery(ctx.Query("since")); since != nil {
		query = query.Where("created_at >= ?", *since)
	}
	if until := parseTimeQuery(ctx.Query("until")); until != nil {
		query = query.Where("created_at <= ?", *until)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	page := bindPage(ctx)
	items := make([]SalesOrder, 0, page.PageSize)
	if err := query.Order("created_at DESC").Offset((page.Page - 1) * page.PageSize).Limit(page.PageSize).Find(&items).Error; err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, gin.H{"items": items, "total": total, "page": page.Page, "page_size": page.PageSize})
}

func (handler *requestHandler) getOrder(ctx *gin.Context) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	order, err := loadSalesOrder(ctx, db, ctx.Param("id"))
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, order)
}

func (handler *requestHandler) createOrder(ctx *gin.Context) {
	var request orderCreateRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidOrder)
		return
	}
	handler.receiveOrder(ctx, request, false)
}

func (handler *requestHandler) importOrder(ctx *gin.Context) {
	principal := handler.runtime.Principal(ctx)
	if principal == nil || strings.TrimSpace(principal.GetPersonAccessToken()) == "" {
		ctx.AbortWithStatusJSON(401, gin.H{"error": "personal access token required", "code": "pat_required"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20))
	if err != nil {
		writeAPIError(ctx, ErrInvalidOrder)
		return
	}
	if !authorizeConnector(ctx, raw) {
		return
	}
	var request orderCreateRequest
	if json.Unmarshal(raw, &request) != nil || strings.TrimSpace(request.ProductID) != "" {
		writeAPIError(ctx, fmt.Errorf("%w: connector must use external_item_id mapping", ErrInvalidOrder))
		return
	}
	if !connectorSourceAllowed(request.Channel, request.Shop) {
		ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "sales connector source is not allowed", "code": "connector_source_denied"})
		return
	}
	handler.receiveOrder(ctx, request, true)
}

func authorizeConnector(ctx *gin.Context, raw []byte) bool {
	shared := os.Getenv(envConnectorSharedToken)
	if shared == "" || strings.TrimSpace(os.Getenv(envConnectorAllowedSources)) == "" {
		ctx.AbortWithStatusJSON(503, gin.H{"error": "sales connector is not configured", "code": "connector_disabled"})
		return false
	}
	provided := ctx.GetHeader("X-LiteLLMOps-Connector-Token")
	expectedDigest, providedDigest := sha256.Sum256([]byte(shared)), sha256.Sum256([]byte(provided))
	if !hmac.Equal(expectedDigest[:], providedDigest[:]) {
		ctx.AbortWithStatusJSON(401, gin.H{"error": "sales connector authentication failed", "code": "connector_auth_failed"})
		return false
	}
	timestampText := strings.TrimSpace(ctx.GetHeader("X-LiteLLMOps-Connector-Timestamp"))
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil || time.Since(time.Unix(timestamp, 0)).Abs() > 5*time.Minute {
		ctx.AbortWithStatusJSON(401, gin.H{"error": "sales connector timestamp is outside the accepted window", "code": "connector_timestamp_invalid"})
		return false
	}
	signature, err := hex.DecodeString(strings.TrimSpace(ctx.GetHeader("X-LiteLLMOps-Connector-Signature")))
	mac := hmac.New(sha256.New, []byte(shared))
	_, _ = mac.Write([]byte(timestampText + "\n"))
	_, _ = mac.Write(raw)
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		ctx.AbortWithStatusJSON(401, gin.H{"error": "sales connector signature is invalid", "code": "connector_signature_invalid"})
		return false
	}
	return true
}

func connectorSourceAllowed(channel, shop string) bool {
	wanted := strings.ToLower(strings.TrimSpace(channel)) + ":" + strings.TrimSpace(shop)
	for _, entry := range strings.Split(os.Getenv(envConnectorAllowedSources), ",") {
		if strings.ToLower(strings.TrimSpace(entry)) == wanted {
			return true
		}
	}
	return false
}

func (handler *requestHandler) receiveOrder(ctx *gin.Context, request orderCreateRequest, trustedImport bool) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	order, _, err := createSalesOrder(ctx, db, request, handler.operator(ctx), trustedImport)
	if err != nil {
		writeAPIError(ctx, err)
		return
	}
	if trustedImport && order.SourceTrust == "trusted" && order.Status == OrderReceived && order.PaymentStatus == "paid" {
		actor := handler.operator(ctx)
		if err = verifySalesOrder(ctx, db, order, actor); err == nil {
			err = matchSalesOrder(ctx, db, order, orderMatchRequest{}, actor)
		}
		if err == nil {
			var product SalesProduct
			if db.First(&product, "id = ?", order.ProductID).Error == nil && product.AutoApply && product.Enabled {
				err = approveSalesOrder(ctx, db, order, actor)
				if err == nil {
					client, clientOK := handler.litellmClient(ctx)
					if !clientOK {
						return
					}
					err = executeSalesOrder(ctx, db, client, order, actor)
				}
			}
		}
	}
	if err != nil {
		if persistedOrderResponse(ctx, db, order) {
			return
		}
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, order)
}

func (handler *requestHandler) verifyOrder(ctx *gin.Context) {
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error { return verifySalesOrder(ctx, db, o, handler.operator(ctx)) })
}
func (handler *requestHandler) matchOrder(ctx *gin.Context) {
	var request orderMatchRequest
	if ctx.ShouldBindJSON(&request) != nil {
		writeAPIError(ctx, ErrInvalidOrder)
		return
	}
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error {
		return matchSalesOrder(ctx, db, o, request, handler.operator(ctx))
	})
}
func (handler *requestHandler) approveOrder(ctx *gin.Context) {
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error { return approveSalesOrder(ctx, db, o, handler.operator(ctx)) })
}
func (handler *requestHandler) executeOrder(ctx *gin.Context) {
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error {
		client, ok := handler.litellmClient(ctx)
		if !ok {
			return errors.New("client unavailable")
		}
		return executeSalesOrder(ctx, db, client, o, handler.operator(ctx))
	})
}
func (handler *requestHandler) reconcileOrder(ctx *gin.Context) {
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error {
		client, ok := handler.litellmClient(ctx)
		if !ok {
			return errors.New("client unavailable")
		}
		return reconcileSalesOrder(ctx, db, client, o, handler.operator(ctx))
	})
}
func (handler *requestHandler) refundReviewOrder(ctx *gin.Context) {
	var request orderRefundReviewRequest
	if ctx.ShouldBindJSON(&request) != nil || strings.TrimSpace(request.Reason) == "" {
		writeAPIError(ctx, fmt.Errorf("%w: refund review reason is required", ErrInvalidOrder))
		return
	}
	handler.orderAction(ctx, func(db *gorm.DB, o *SalesOrder) error {
		return transitionOrder(ctx, db, o,
			refundReviewAllowedStatuses(),
			map[string]any{"status": OrderRefundReview, "note": strings.TrimSpace(request.Reason)},
			"refund_review", "manual_review_required", handler.operator(ctx))
	})
}

func (handler *requestHandler) orderAction(ctx *gin.Context, action func(*gorm.DB, *SalesOrder) error) {
	db, ok := handler.database(ctx)
	if !ok {
		return
	}
	order, err := loadSalesOrder(ctx, db, ctx.Param("id"))
	if err == nil {
		err = action(db, order)
	}
	if err != nil {
		if persistedOrderResponse(ctx, db, order) {
			return
		}
		writeAPIError(ctx, err)
		return
	}
	ctx.JSON(200, order)
}

func persistedOrderResponse(ctx *gin.Context, db *gorm.DB, order *SalesOrder) bool {
	if order == nil || strings.TrimSpace(order.ID) == "" {
		return false
	}
	if err := db.WithContext(ctx.Request.Context()).First(order, "id = ?", order.ID).Error; err != nil {
		return false
	}
	switch order.Status {
	case OrderAppliedUnverified, OrderRetryableFailed, OrderReconcileRequired:
		ctx.JSON(http.StatusAccepted, order)
		return true
	default:
		return false
	}
}
