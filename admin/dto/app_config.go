package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/11 17:36:42
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/11 17:36:42
 */

type AppConfigGroupRequest struct {
	Group string `uri:"group" binding:"required"`
}

type AppConfigControlRequest struct {
	Group string         `uri:"group" binding:"required" swaggerignore:"true"`
	Data  map[string]any `json:"data" binding:"required"`
}

// AppConfigSecurityProfile documents the non-secret public authentication
// capability projection returned to login and registration screens.
type AppConfigSecurityProfile struct {
	EmailChallengeReady bool `json:"emailChallengeReady"`
	EmailEnabled        bool `json:"emailEnabled"`
	GitHubEnabled       bool `json:"githubEnabled"`
	LarkEnabled         bool `json:"larkEnabled"`
	PhoneEnabled        bool `json:"phoneEnabled"`
	RegisterEnabled     bool `json:"registerEnabled"`
}

// AppConfigPublicProfile is the OpenAPI shape of the public profile. Groups
// remain extensible maps, while security has a typed contract for the runtime
// email challenge capability consumed by the frontend.
type AppConfigPublicProfile struct {
	Base     map[string]any           `json:"base,omitempty"`
	Security AppConfigSecurityProfile `json:"security"`
	Theme    map[string]any           `json:"theme,omitempty"`
}
