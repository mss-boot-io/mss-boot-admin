package dto

import (
	"github.com/mss-boot-io/mss-boot-admin/pkg"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 16:43:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 16:43:12
 */

type OauthGetLoginURLReq struct {
	State string `query:"state" form:"state" binding:"required"`
}

type OauthCallbackReq struct {
	Provider pkg.LoginProvider `uri:"provider" binding:"required"`
	Code     string            `query:"code" form:"code" binding:"required"`
	State    string            `query:"state" form:"state" binding:"required"`
}

type GithubControlReq struct {
	//github密码或者token
	Password string `json:"password" binding:"required"`
}

type GithubGetResp struct {
	//github邮箱
	Email string `json:"email" bson:"email"`
	//已配置
	Configured bool `json:"configured" bson:"configured"`
	//创建时间
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
	//更新时间
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

type OauthToken struct {
	// Provider is the name of the OAuth2 provider[GitHub, Lark].
	Provider string `uri:"provider" binding:"required"`
	// AccessToken is the token that authorizes and authenticates
	// the requests.
	AccessToken string `json:"accessToken"`

	// TokenType is the type of token.
	// The Type method returns either this or "Bearer", the default.
	TokenType string `json:"tokenType,omitempty"`

	// RefreshToken is a token that's used by the application
	// (as opposed to the user) to refresh the access token
	// if it expires.
	RefreshToken string `json:"refreshToken,omitempty"`

	// Expiry is the optional expiration time of the access token.
	//
	// If zero, TokenSource implementations will reuse the same
	// token forever and RefreshToken or equivalent
	// mechanisms for that TokenSource will not be used.
	Expiry *time.Time `json:"expiry,omitempty"`

	RefreshExpiry *time.Time `json:"refreshExpiry,omitempty"`
}
