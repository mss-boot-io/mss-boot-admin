package models

import (
	"strings"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"

	"github.com/google/uuid"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 19:42:35
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 19:42:35
 */

// TaskRun support http/grpc/script status: 0: unknown, 1: success, 2: fail, 3: running
type TaskRun struct {
	ID        string      `gorm:"primarykey" json:"id" form:"id" query:"id"`
	CreatedAt time.Time   `json:"createdAt"`
	TaskID    string      `json:"taskID" gorm:"index;foreignKey:TaskID;references:ID"`
	Status    enum.Status `json:"status"`
}

func (*TaskRun) TableName() string {
	return "mss_boot_task_runs"
}

func (t *TaskRun) BeforeCreate(_ *gorm.DB) (err error) {
	t.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
	return nil
}

func (t *TaskRun) Write(p []byte) (int, error) {
	log := &TaskRunLog{
		TaskRunID: t.ID,
		Content:   string(p),
	}
	err := gormdb.DB.Create(log).Error
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

type TaskRunLog struct {
	ID        string    `gorm:"primarykey" json:"id" form:"id" query:"id"`
	TaskRunID string    `json:"taskRunID" gorm:"index;foreignKey:TaskRunID;references:ID"`
	CreatedAt time.Time `json:"createdAt"`
	Content   string    `json:"content" gorm:"type:text"`
}

func (*TaskRunLog) TableName() string {
	return "mss_boot_task_run_logs"
}

func (l *TaskRunLog) BeforeCreate(_ *gorm.DB) (err error) {
	l.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
	return nil
}
