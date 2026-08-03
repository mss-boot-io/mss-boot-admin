package config

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/10 14:06
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/10 14:06
 */

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OAuth2 holds the configuration for the OAuth2 provider.
type OAuth2 struct {
	Issuer       string   `yaml:"issuer" json:"issuer"`
	Endpoint     Endpoint `yaml:"endpoint" json:"endpoint"`
	ClientID     string   `yaml:"clientID" json:"clientID"`
	ClientSecret string   `yaml:"clientSecret" json:"clientSecret"`
	Scopes       []string `yaml:"scopes" json:"scopes"`
	RedirectURL  string   `yaml:"redirectURL" json:"redirectURL"`
	AllowGroup   []string `yaml:"allowGroup" json:"allowGroup"`
}

type Endpoint struct {
	AuthURL       string `yaml:"authURL" json:"authURL"`
	DeviceAuthURL string `yaml:"deviceAuthURL" json:"deviceAuthURL"`
	TokenURL      string `yaml:"tokenURL" json:"tokenURL"`

	// AuthStyle optionally specifies how the endpoint wants the
	// client ID & client secret sent. The zero value means to
	// auto-detect.
	AuthStyle int `yaml:"authStyle" json:"authStyle"`
}

// GetIssuer returns the OAuth2 issuer.
func (e *OAuth2) GetIssuer() string {
	return e.Issuer
}

// GetClientID returns the OAuth2 client ID.
func (e *OAuth2) GetClientID() string {
	return e.ClientID
}

// GetClientSecret returns the OAuth2 client secret.
func (e *OAuth2) GetClientSecret() string {
	return e.ClientSecret
}

// GetScopes returns the OAuth2 scopes.
func (e *OAuth2) GetScopes() []string {
	return e.Scopes
}

// GetRedirectURL returns the OAuth2 redirect URL.
func (e *OAuth2) GetRedirectURL() string {
	return e.RedirectURL
}

// GetOAuth2Config returns an oauth2.Config.
func (e *OAuth2) GetOAuth2Config(c context.Context) (*oauth2.Config, error) {
	conf := &oauth2.Config{
		ClientID:     e.ClientID,
		ClientSecret: e.ClientSecret,
		Scopes:       e.Scopes,
		RedirectURL:  e.RedirectURL,
	}
	if e.Issuer != "" {
		provider, err := oidc.NewProvider(c, e.Issuer)
		if err != nil {
			return nil, err
		}
		conf.Endpoint = provider.Endpoint()
		return conf, nil
	}
	conf.Endpoint.AuthURL = e.Endpoint.AuthURL
	conf.Endpoint.TokenURL = e.Endpoint.TokenURL
	conf.Endpoint.AuthStyle = oauth2.AuthStyle(e.Endpoint.AuthStyle)
	conf.Endpoint.DeviceAuthURL = e.Endpoint.DeviceAuthURL
	return conf, nil
}
