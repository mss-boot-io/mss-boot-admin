package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/2 00:53:53
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/2 00:53:53
 */

type UserConfigGroupRequest struct {
	Group string `uri:"group" binding:"required"`
}

type UserConfigControlRequest struct {
	Group string         `uri:"group" binding:"required" swaggerignore:"true"`
	Data  map[string]any `json:"data" binding:"required"`
}
