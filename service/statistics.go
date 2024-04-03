package service

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/12 17:50:50
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/12 17:50:50
 */

type Statistics struct{}

func (*Statistics) Get(ctx *gin.Context, name string) (*dto.StatisticsGetResponse, error) {
	list := make([]*models.Statistics, 0)
	err := center.GetDB(ctx, &models.Statistics{}).Where("name = ?", name).Find(&list).Error
	if err != nil {
		return nil, err
	}
	result := &dto.StatisticsGetResponse{
		Name:  name,
		Items: make([]dto.StatisticsItem, len(list)),
	}
	for i := range list {
		result.Items[i].Date = list[i].Time
		result.Items[i].Scales = float64(list[i].Value) / 100
	}
	return result, nil
}
