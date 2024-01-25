package models

import (
	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/center"
	"gorm.io/gorm"
	"log/slog"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/12 15:16:38
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/12 15:16:38
 */

type Statistics struct {
	ModelGormTenant
	// Name 统计名称
	Name string `gorm:"column:name;type:varchar(255);not null;comment:统计名称" json:"name"`
	// Type 统计类型
	Type string `gorm:"column:type;type:varchar(255);not null;comment:统计类型" json:"type"`
	// Value 统计值 * 100
	Value int `gorm:"column:value;type:int;not null;comment:统计值 * 100" json:"value"`
	// Time 统计时间
	Time string `gorm:"column:time;type:varchar(50);not null;comment:统计时间" json:"time"`
}

func (*Statistics) TableName() string {
	return "mss_boot_statistics"
}

func (e *Statistics) Calibrate(ctx *gin.Context, object center.StatisticsObject) error {
	s := &Statistics{
		Name: object.StatisticsName(),
		Type: object.StatisticsType(),
		Time: object.StatisticsTime(),
	}
	err := center.GetDB(ctx, s).Where(s).
		FirstOrCreate(s).Error
	if err != nil {
		slog.Error("Statistics Calibrate", "error", err)
		return err
	}
	s.Value, err = object.StatisticsCalibrate()
	if err != nil {
		slog.Error("Statistics Calibrate", "error", err)
		return err
	}
	err = center.GetDB(ctx, s).Save(s).Error
	if err != nil {
		slog.Error("Statistics Calibrate", "error", err)
		return err
	}
	return nil
}

func (e *Statistics) Increase(ctx *gin.Context, object center.StatisticsObject) error {
	s := &Statistics{
		Name: object.StatisticsName(),
		Type: object.StatisticsType(),
		Time: object.StatisticsTime(),
	}
	err := center.GetDB(ctx, s).Where(s).
		FirstOrCreate(s).Error
	if err != nil {
		slog.Error("Statistics Increase", "error", err)
		return err
	}
	if err != nil {
		slog.Error("Statistics Increase", "error", err)
		return err
	}
	err = center.GetDB(ctx, s).Model(e).
		Update("value", gorm.Expr("value + ?", object.StatisticsStep())).Error
	if err != nil {
		slog.Error("Statistics Increase", "error", err)
		return err
	}
	return nil
}

func (e *Statistics) Reduce(ctx *gin.Context, object center.StatisticsObject) error {
	s := &Statistics{
		Name: object.StatisticsName(),
		Type: object.StatisticsType(),
		Time: object.StatisticsTime(),
	}
	err := center.GetDB(ctx, s).Where(s).
		FirstOrCreate(s).Error
	if err != nil {
		slog.Error("Statistics Reduce", "error", err)
		return err
	}
	if err != nil {
		slog.Error("Statistics Reduce", "error", err)
		return err
	}
	err = center.GetDB(ctx, s).Model(s).
		Update("value", gorm.Expr("value - ?", object.StatisticsStep())).Error
	if err != nil {
		slog.Error("Statistics Reduce", "error", err)
		return err
	}
	return nil
}
