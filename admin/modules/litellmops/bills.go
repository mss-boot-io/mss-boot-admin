package litellmops

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// EnvBillsDSN configures the read-only TimescaleDB connection used for bills.
const EnvBillsDSN = "LITELLMOPS_BILLS_DSN"

// SpendLog is the billing row projection from LiteLLM_SpendLogs.
type SpendLog struct {
	RequestID        string    `gorm:"column:request_id" json:"request_id"`
	Model            string    `gorm:"column:model" json:"model"`
	Status           string    `gorm:"column:status" json:"status"`
	Spend            float64   `gorm:"column:spend" json:"spend"`
	PromptTokens     int64     `gorm:"column:prompt_tokens" json:"prompt_tokens"`
	CompletionTokens int64     `gorm:"column:completion_tokens" json:"completion_tokens"`
	TotalTokens      int64     `gorm:"column:total_tokens" json:"total_tokens"`
	StartTime        time.Time `gorm:"column:startTime" json:"start_time"`
	APIKey           string    `gorm:"column:api_key" json:"-"`
}

// TableName implements gorm.Tabler.
func (SpendLog) TableName() string { return "LiteLLM_SpendLogs" }

// BillsQuery filters the billing ledger.
type BillsQuery struct {
	Model         string
	Status        string
	KeyHashPrefix string
	Since         *time.Time
	Until         *time.Time
	Page          int
	PageSize      int
}

// BillsPage is one page of ledger rows plus aggregates over the full filter.
type BillsPage struct {
	Items    []SpendLog `json:"items"`
	Total    int64      `json:"total"`
	SpendSum float64    `json:"spend_sum"`
	TokenSum int64      `json:"token_sum"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// BillsService queries the LiteLLM spend ledger read-only.
type BillsService struct {
	db *gorm.DB
}

// NewBillsService requires an explicit ledger database handle.
func NewBillsService(db *gorm.DB) (*BillsService, error) {
	if db == nil {
		return nil, errors.New("litellmops bills database is required")
	}
	return &BillsService{db: db}, nil
}

// List returns one filtered page and aggregate sums over the whole filter.
func (service *BillsService) List(ctx context.Context, query BillsQuery) (*BillsPage, error) {
	if ctx == nil {
		return nil, errors.New("litellmops bills context is required")
	}
	filtered := service.filtered(ctx, query)

	var total int64
	if err := filtered.Session(&gorm.Session{}).Model(&SpendLog{}).Distinct("request_id").Count(&total).Error; err != nil {
		return nil, err
	}
	type sums struct {
		Spend float64
		Token int64
	}
	var aggregate sums
	if err := filtered.Session(&gorm.Session{}).Model(&SpendLog{}).
		Select("COALESCE(SUM(spend), 0) AS spend, COALESCE(SUM(total_tokens), 0) AS token").
		Scan(&aggregate).Error; err != nil {
		return nil, err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items := make([]SpendLog, 0, pageSize)
	if err := filtered.Session(&gorm.Session{}).
		Order(`"startTime" DESC`).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return &BillsPage{
		Items:    items,
		Total:    total,
		SpendSum: aggregate.Spend,
		TokenSum: aggregate.Token,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (service *BillsService) filtered(ctx context.Context, query BillsQuery) *gorm.DB {
	db := service.db.WithContext(ctx).Model(&SpendLog{})
	if model := strings.TrimSpace(query.Model); model != "" {
		db = db.Where("model = ?", model)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if prefix := strings.TrimSpace(query.KeyHashPrefix); prefix != "" {
		db = db.Where("api_key LIKE ?", prefix+"%")
	}
	if query.Since != nil {
		db = db.Where(`"startTime" >= ?`, *query.Since)
	}
	if query.Until != nil {
		db = db.Where(`"startTime" < ?`, *query.Until)
	}
	return db
}

var (
	billsDB     *gorm.DB
	billsDBErr  error
	billsDBOnce sync.Once
)

// billsDatabase opens the read-only ledger connection lazily from the
// deployment environment. The DSN must belong to a SELECT-only role.
func billsDatabase(open func(dsn string) (*gorm.DB, error)) (*gorm.DB, error) {
	billsDBOnce.Do(func() {
		dsn := strings.TrimSpace(os.Getenv(EnvBillsDSN))
		if dsn == "" {
			billsDBErr = errors.New("litellmops bills DSN is not configured (" + EnvBillsDSN + ")")
			return
		}
		billsDB, billsDBErr = open(dsn)
	})
	return billsDB, billsDBErr
}
