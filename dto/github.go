package dto

import "time"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2022/10/19 16:43:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2022/10/19 16:43:12
 */

type GithubGetLoginURLReq struct {
	State string `query:"state" form:"state" binding:"required"`
}

type GithubCallbackReq struct {
	Code  string `query:"code" form:"code" binding:"required"`
	State string `query:"state" form:"state" binding:"required"`
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
