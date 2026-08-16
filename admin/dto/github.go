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

type OauthCallbackReq struct {
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
	Token     string            `json:"token,omitempty"`
	Expire    *time.Time        `json:"expire,omitempty"`
}

type OauthToken struct {
	// Provider is the name of the OAuth2 provider[GitHub, Lark].
	Provider string `json:"-"`
	// Intent is the server-validated flow purpose. Clients must not infer the
	// purpose from a caller-controlled state prefix.
	Intent string `json:"-"`
	// AccessToken is the token that authorizes and authenticates
	// the requests.
	AccessToken string `json:"-"`

	// TokenType is the type of token.
	// The Type method returns either this or "Bearer", the default.
	TokenType string `json:"-"`

	// RefreshToken is a token that's used by the application
	// (as opposed to the user) to refresh the access token
	// if it expires.
	RefreshToken string `json:"-"`

	// Expiry is the optional expiration time of the access token.
	//
	// If zero, TokenSource implementations will reuse the same
	// token forever and RefreshToken or equivalent
	// mechanisms for that TokenSource will not be used.
	Expiry *time.Time `json:"-"`

	RefreshExpiry *time.Time `json:"-"`
}
