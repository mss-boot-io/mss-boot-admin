// Package litellmops implements the LiteLLM operations module: snapshots of
// LiteLLM users and virtual keys, manual recharge via the LiteLLM Admin API,
// and billing queries from a read-only TimescaleDB connection. LiteLLM remains
// the budget authority; this module never writes the LiteLLM database.
//
// The module is hand-written because the 1.3.x generator checkpoints only
// support string/enum/bool, non-nullable, full-CRUD verticals, which cannot
// express this integration/reporting module. Structure follows the generated
// supplier module contract (business.Module, migrations, casbin authorizer).
package litellmops

import (
	"errors"

	"github.com/mss-boot-io/mss-boot-admin/admin/business"
)

// Permission codes enforced on every route by the casbin authorizer.
const (
	PermissionUserList          = "litellmops:user-list"
	PermissionUserRead          = "litellmops:user-read"
	PermissionKeyList           = "litellmops:key-list"
	PermissionKeyRead           = "litellmops:key-read"
	PermissionSync              = "litellmops:sync"
	PermissionBills             = "litellmops:bills"
	PermissionRecharge          = "litellmops:recharge"
	PermissionOrgList           = "litellmops:org-list"
	PermissionOrgRead           = "litellmops:org-read"
	PermissionOrgWrite          = "litellmops:org-write"
	PermissionProductRead       = "litellmops:product-read"
	PermissionProductWrite      = "litellmops:product-write"
	PermissionOrderRead         = "litellmops:order-read"
	PermissionOrderImport       = "litellmops:order-import"
	PermissionOrderVerify       = "litellmops:order-verify"
	PermissionOrderApprove      = "litellmops:order-approve"
	PermissionOrderExecute      = "litellmops:order-execute"
	PermissionOrderReconcile    = "litellmops:order-reconcile"
	PermissionOrderRefundReview = "litellmops:order-refund-review"
	PermissionUserWrite         = "litellmops:user-write"
	PermissionKeyIssue          = "litellmops:key-issue"
	PermissionKeyWrite          = "litellmops:key-write"
	PermissionKeyRevoke         = "litellmops:key-revoke"
	PermissionGatewayRead       = "litellmops:gateway-read"
	PermissionGatewayWrite      = "litellmops:gateway-write"
	PermissionManagementResolve = "litellmops:management-resolve"
)

// ModuleName is the stable registry identifier.
const ModuleName = "litellmops"

type businessModule struct{}

// Module returns this module's explicit Admin composition contract.
func Module() business.Module { return businessModule{} }

func (businessModule) Name() string { return ModuleName }

func (businessModule) Register(registry *business.Registry) error {
	if registry == nil {
		return errors.New("litellmops business registry is required")
	}
	return registry.Register(business.Registration{
		Descriptor:    descriptor(),
		Migrations:    RegisterMigration,
		Readiness:     verifyRuntimeReadiness,
		Routes:        registerBusinessRoutes,
		Presentations: nil,
	})
}

func descriptor() business.Descriptor {
	return business.Descriptor{
		Name:        ModuleName,
		DisplayName: "LiteLLM 运营",
		Description: "LiteLLM 用户/Key 同步视图、手工充值与账单查询；写路径只走 LiteLLM Admin API。",
		Version:     "v1alpha1",
		Model:       new(UserSnapshot),
		Permissions: []business.Permission{
			{Code: PermissionUserList, DisplayName: "查看 LiteLLM 用户额度列表", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionUserRead, DisplayName: "查看 LiteLLM 用户额度详情", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionKeyList, DisplayName: "查看 LiteLLM Key 快照列表", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionKeyRead, DisplayName: "查看 LiteLLM Key 快照详情", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionSync, DisplayName: "触发 LiteLLM 同步", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionBills, DisplayName: "查询 LiteLLM 账单", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionRecharge, DisplayName: "手工充值 LiteLLM 用户额度", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrgList, DisplayName: "查看 LiteLLM 组织", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionOrgRead, DisplayName: "查看 LiteLLM 组织详情", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionOrgWrite, DisplayName: "管理 LiteLLM 组织与团队", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionProductRead, DisplayName: "查看渠道商品映射", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionProductWrite, DisplayName: "管理渠道商品映射", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderRead, DisplayName: "查看销售订单", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionOrderImport, DisplayName: "创建和导入销售订单", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderVerify, DisplayName: "核验和匹配销售订单", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderApprove, DisplayName: "批准销售订单", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderExecute, DisplayName: "执行销售订单充值", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderReconcile, DisplayName: "对账销售订单", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionOrderRefundReview, DisplayName: "提交退款人工复核", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionUserWrite, DisplayName: "管理 LiteLLM 用户", DefaultRoles: []string{"admin"}},
			{Code: PermissionKeyIssue, DisplayName: "签发 LiteLLM Key", DefaultRoles: []string{"admin"}},
			{Code: PermissionKeyWrite, DisplayName: "更新 LiteLLM Key", DefaultRoles: []string{"admin"}},
			{Code: PermissionKeyRevoke, DisplayName: "停用或删除 LiteLLM Key", DefaultRoles: []string{"admin"}},
			{Code: PermissionGatewayRead, DisplayName: "查看 LiteLLM 网关状态", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionGatewayWrite, DisplayName: "管理 LiteLLM 模型", DefaultRoles: []string{"admin"}},
			{Code: PermissionManagementResolve, DisplayName: "对账和结案不确定管理命令", DefaultRoles: []string{"admin"}},
		},
		Menu: business.Menu{
			Path:          "/litellm-ops",
			DisplayName:   "LiteLLM 运营",
			DisplayNameEn: "LiteLLM Ops",
			Icon:          "wallet",
			Order:         90,
			Hidden:        false,
		},
	}
}
