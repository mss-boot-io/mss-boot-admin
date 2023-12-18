package dto

import (
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"time"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 22:16:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 22:16:32
 */

type UserSearch struct {
	actions.Pagination `search:"inline"`
	// ID
	ID string `query:"id" form:"id" search:"type:contains;column:id"`
	//名称
	Name string `query:"name" form:"name" search:"type:contains;column:name"`
}

type LoginResponse struct {
	Code   int       `json:"code"`
	Expire time.Time `json:"expire"`
	Token  string    `json:"token"`
}

type FakeCaptchaRequest struct {
	Phone string `json:"phone" binding:"required"`
}

type FakeCaptchaResponse struct {
	Code   int8   `json:"code"`
	Status string `json:"status"`
}

type PasswordResetRequest struct {
	UserID   string `uri:"userID" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateUserInfoRequest struct {
	// Name 昵称
	Name string `json:"name"`
	// Email 邮箱
	Email string `json:"email"`
	// Avatar 头像
	Avatar string `json:"avatar"`
	// Signature 个性签名
	Signature string `json:"signature"`
	// Title 职位
	Title string `json:"title"`
	// Group 组别
	Group string `json:"group"`
	// Country 国家
	Country string `json:"country"`
	// Province 省份
	Province string `json:"province"`
	// City 城市
	City string `json:"city"`
	// Address 地址
	Address string `json:"address"`
	// Phone 手机号
	Phone string `json:"phone"`
	// Profile 个人简介
	Profile string `json:"profile"`
	// Tags 标签
	Tags []string `json:"tags"`
}

type UpdateAvatarResponse struct {
	Avatar string `json:"avatar"`
}
