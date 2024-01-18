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
	Group string         `uri:"group" binding:"required"`
	Data  map[string]any `json:"data" binding:"required"`
}
