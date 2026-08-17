package dto

import (
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
)

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Captcha  string `json:"captcha" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterResponse struct {
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	Captcha  string `json:"captcha"`
	Password string `json:"password" binding:"required"`
}

type AccountReauthenticationRequest struct {
	Method   string `json:"method" binding:"required,oneof=password"`
	Password string `json:"password" binding:"required"`
}

type AccountPasswordChangeRequest struct {
	NewPassword string `json:"newPassword" binding:"required"`
}

type AccountSecurityStatus struct {
	HasLocalPassword              bool       `json:"hasLocalPassword"`
	RecentAuthentication          bool       `json:"recentAuthentication"`
	RecentAuthenticationExpiresAt *time.Time `json:"recentAuthenticationExpiresAt,omitempty"`
	ReauthenticationLockedUntil   *time.Time `json:"reauthenticationLockedUntil,omitempty"`
}

type AccountPasswordChangeResponse struct {
	SignedOut bool `json:"signedOut"`
}

type UserSearch struct {
	actions.Pagination `search:"inline"`
	ID                 string `query:"id" form:"id" search:"type:contains;column:id"`
	Name               string `query:"name" form:"name" search:"type:contains;column:name"`
}

// BrowserSessionResponse intentionally omits the Admin JWT. The browser owns
// only an HttpOnly cookie and a separate readable CSRF token.
type BrowserSessionResponse struct {
	Code   int       `json:"code"`
	Expire time.Time `json:"expire"`
}

type FakeCaptchaRequest struct {
	Email string `json:"email" binding:"required,email"`
	UseBy string `json:"useBy" binding:"required,oneof=register login resetPassword"`
}

type FakeCaptchaResponse struct {
	Status string `json:"status"`
}

type PasswordResetRequest struct {
	UserID   string `uri:"userID" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateUserInfoRequest struct {
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Avatar    string   `json:"avatar"`
	Signature string   `json:"signature"`
	Title     string   `json:"title"`
	Group     string   `json:"group"`
	Country   string   `json:"country"`
	Province  string   `json:"province"`
	City      string   `json:"city"`
	Address   string   `json:"address"`
	Phone     string   `json:"phone"`
	Profile   string   `json:"profile"`
	Tags      []string `json:"tags"`
}

type UpdateAvatarResponse struct {
	Avatar string `json:"avatar"`
}
