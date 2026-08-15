package dto

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/12 12:08:11
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/12 12:08:11
 */

type LanguageSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name" binding:"max=255"`
	// Status 状态
	Status string `query:"status" form:"status" search:"type:exact;column:status" binding:"omitempty,oneof=enabled disabled"`
	// View summary omits the potentially large definition payload from list rows.
	View string `query:"view" form:"view" binding:"omitempty,oneof=summary full"`
}

// LanguageWriteRequest is the complete client-owned write surface. Record and
// definition identifiers, timestamps, and tenant metadata remain server-owned.
type LanguageWriteRequest struct {
	Name              string                  `json:"name"`
	Remark            string                  `json:"remark"`
	Status            string                  `json:"status"`
	Defines           *models.LanguageDefines `json:"defines"`
	ExpectedUpdatedAt string                  `json:"expectedUpdatedAt"`
}

// LanguageSummary is the bounded list projection. Defines are loaded only by
// the detail endpoint so list requests cannot amplify the JSON definition body.
type LanguageSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PublicLanguage omits management metadata while retaining the legacy public
// definition array used by clients that do not consume the flattened profile.
type PublicLanguage struct {
	Name    string                 `json:"name"`
	Defines models.LanguageDefines `json:"defines"`
}
