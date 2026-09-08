// Package litellmops implements the LiteLLM operations module: read-only
// snapshots of LiteLLM users and virtual keys, a manual sync endpoint backed
// by the LiteLLM Admin API, and billing queries served from a read-only
// TimescaleDB connection. LiteLLM remains the single source of truth; this
// module never writes to it.
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
	PermissionUserList = "litellmops:user-list"
	PermissionUserRead = "litellmops:user-read"
	PermissionKeyList  = "litellmops:key-list"
	PermissionKeyRead  = "litellmops:key-read"
	PermissionSync     = "litellmops:sync"
	PermissionBills    = "litellmops:bills"
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
		Description: "LiteLLM 用户/Key 只读同步视图与账单查询；LiteLLM 为唯一权威。",
		Version:     "v1alpha1",
		Model:       new(UserSnapshot),
		Permissions: []business.Permission{
			{Code: PermissionUserList, DisplayName: "查看 LiteLLM 用户额度列表", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionUserRead, DisplayName: "查看 LiteLLM 用户额度详情", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionKeyList, DisplayName: "查看 LiteLLM Key 快照列表", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionKeyRead, DisplayName: "查看 LiteLLM Key 快照详情", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
			{Code: PermissionSync, DisplayName: "触发 LiteLLM 同步", DefaultRoles: []string{"admin", "litellmops-finance"}},
			{Code: PermissionBills, DisplayName: "查询 LiteLLM 账单", DefaultRoles: []string{"admin", "litellmops-finance", "litellmops-readonly"}},
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
