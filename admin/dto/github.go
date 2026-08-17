package dto

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 16:43:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 16:43:12
 */

type OAuthCallbackRequest struct {
	Provider pkg.LoginProvider `uri:"provider" json:"-" binding:"required"`
	Code     string            `json:"code" binding:"required"`
	State    string            `json:"state" binding:"required"`
}

type OAuthAuthorizeRequest struct {
	Provider pkg.LoginProvider `json:"provider" binding:"required"`
	Intent   string            `json:"intent" binding:"required,oneof=login binding reauthentication"`
}

type OAuthAuthorizeResponse struct {
	AuthorizeURL string    `json:"authorizeURL"`
	AttemptID    string    `json:"attemptID"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// OAuthCallbackResponse contains only Admin-owned values. Provider access and
// refresh tokens are intentionally absent from this public contract.
type OAuthCallbackResponse struct {
	Code      int               `json:"code"`
	Provider  pkg.LoginProvider `json:"provider"`
	Intent    string            `json:"intent"`
	AttemptID string            `json:"attemptID"`
	Expire    *time.Time        `json:"expire,omitempty"`
}
