package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/25 17:03:45
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/25 17:03:45
 */

type GetAuthorizeRequest struct {
	RoleID string `uri:"roleID" binding:"required"`
}

type UpdateAuthorizeRequest struct {
	GetAuthorizeRequest
	Keys []string `json:"keys" binding:"required"`
}
