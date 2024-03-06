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
	Group string                          `uri:"group" binding:"required" swaggerignore:"true"`
	Data  map[string]AppConfigControlItem `json:"data" binding:"required"`
}

type AppConfigControlItem struct {
	Auth  bool `json:"auth"`
	Value any  `json:"value"`
}
