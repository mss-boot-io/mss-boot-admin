package dto

import (
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
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

type UserSearch struct {
	actions.Pagination `search:"inline"`
	ID                 string `query:"id" form:"id" search:"type:contains;column:id"`
	Name               string `query:"name" form:"name" search:"type:contains;column:name"`
}

type LoginResponse struct {
	Code   int       `json:"code"`
	Expire time.Time `json:"expire"`
	Token  string    `json:"token"`
}

type FakeCaptchaRequest struct {
	Phone string `json:"phone"`
	Email string `json:"email"`
	UseBy string `json:"useBy"`
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
