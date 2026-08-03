package store

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/4/21 17:20
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/4/21 17:20
 */

import (
	"context"

	"golang.org/x/oauth2"
)

// DefaultOAuth2Store default oauth2 store
var DefaultOAuth2Store OAuth2Store

// OAuth2Store is the interface for OAuth2 configuration.
type OAuth2Store interface {
	GetClientByDomain(c context.Context, domain string) (OAuth2Configure, error)
}

// OAuth2Configure is the interface for OAuth2 configuration.
type OAuth2Configure interface {
	GetOAuth2Config(c context.Context) (*oauth2.Config, error)
	GetIssuer() string
	GetClientID() string
	GetClientSecret() string
	GetRedirectURL() string
	GetScopes() []string
}
