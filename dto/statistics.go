package dto

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/12 17:53:32
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/12 17:53:32
 */

type StatisticsGetRequest struct {
	Name string `uri:"name" binding:"required"`
}

type StatisticsGetResponse struct {
	Name  string           `json:"name"`
	Type  string           `json:"type"`
	Items []StatisticsItem `json:"items"`
}

type StatisticsItem struct {
	Date   string  `json:"time"`
	Scales float64 `json:"scales"`
}
