package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mss-boot-io/mss-boot-admin-api/pkg"
	"github.com/mss-boot-io/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"log/slog"
	"time"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 16:11:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 16:11:48
 */

// Task support http/grpc/script
type Task struct {
	actions.ModelGorm
	Name       string       `json:"name"`
	EntryID    int          `json:"entryID"`
	Spec       string       `json:"spec"`
	Command    string       `json:"command"`
	Args       Args         `json:"args" swaggertype:"array,string" gorm:"type:text"`
	Protocol   string       `json:"-" gorm:"-"`
	Endpoint   string       `json:"endpoint"`
	Body       string       `json:"body" gorm:"type:bytes"`
	Status     enum.Status  `json:"status"`
	Remark     string       `json:"remark" gorm:"type:text"`
	CheckedAtR *time.Time   `gorm:"-" json:"checkedAt"`
	CheckedAt  sql.NullTime `gorm:"index" swaggertype:"string" json:"-"`
	Timeout    int          `json:"timeout"`
	Method     string       `gorm:"size:10" json:"method"`
	Python     string       `json:"python"`
	Metadata   string       `json:"metadata" gorm:"type:bytes"`
}

func (*Task) TableName() string {
	return "mss_boot_tasks"
}

func (t *Task) AfterFind(tx *gorm.DB) (err error) {
	if t.CheckedAt.Valid {
		t.CheckedAtR = &t.CheckedAt.Time
	}
	return
}

func (t *Task) BeforeDelete(_ *gorm.DB) (err error) {
	return task.RemoveJob(t.ID)
}

// Run task todo
func (t *Task) Run() {
	t.CheckedAt = sql.NullTime{
		Time:  time.Now(),
		Valid: true,
	}
	err := gormdb.DB.Updates(t).Error
	if err != nil {
		slog.Error("task run update task error", slog.Any("err", err))
		return
	}

	taskRun := &TaskRun{
		TaskID: t.ID,
		Status: enum.Locked,
	}
	task := &pkg.Task{
		ID:       t.ID,
		Name:     t.Name,
		Endpoint: t.Endpoint,
		Method:   t.Method,
		Command:  t.Command,
		Args:     t.Args,
		Python:   t.Python,
		Writer:   taskRun,
		Metadata: make(map[string]string),
		Timeout:  time.Duration(t.Timeout) * time.Second,
	}
	if t.Metadata != "" {
		err := json.Unmarshal([]byte(t.Metadata), &task.Metadata)
		if err != nil {
			slog.Error("task metadata unmarshal error", slog.Any("err", err))
		}
	}
	err = gormdb.DB.Create(taskRun).Error
	if err != nil {
		slog.Error("task run create task run error", slog.Any("err", err))
		return
	}
	err = task.Run()
	taskRun.Status = enum.Enabled
	if err != nil {
		slog.Error("task run error", slog.Any("err", err))
		taskRun.Status = enum.Disabled
	}
	err = gormdb.DB.Updates(taskRun).Error
	if err != nil {
		slog.Error("task run update task run error", slog.Any("err", err))
	}
}

type TaskStorage struct {
	DB      *gorm.DB
	Spec    string
	job     cron.Job
	entryID cron.EntryID
}

func (t *TaskStorage) Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error) {
	if key == "task" {
		return t.entryID, t.Spec, t.job, true, nil
	}
	if t.DB == nil {
		err = fmt.Errorf("db is nil")
		return
	}
	task := &Task{}
	if err = t.DB.Where("id = ?", key).First(task).Error; err != nil {
		return
	}
	return cron.EntryID(task.EntryID), task.Spec, task, true, nil
}

func (t *TaskStorage) Set(key string, entryID cron.EntryID, spec string, job cron.Job) error {
	if key == "task" {
		t.Spec = spec
		t.job = job
		return nil
	}
	if t.DB == nil {
		return fmt.Errorf("db is nil")
	}
	task := &Task{}
	err := t.DB.Where("id = ?", key).First(task).Error
	if err != nil {
		return err
	}
	task.EntryID = int(entryID)
	task.Spec = spec
	return t.DB.Updates(task).Error
}

func (t *TaskStorage) Update(key string, entryID cron.EntryID) error {
	if key == "task" {
		t.entryID = entryID
		return nil
	}
	if t.DB == nil {
		return fmt.Errorf("db is nil")
	}
	task := &Task{}
	err := t.DB.Where("id = ?", key).First(task).Error
	if err != nil {
		return err
	}
	task.EntryID = int(entryID)
	return t.DB.Updates(task).Error
}

func (t *TaskStorage) Remove(key string) error {
	if key == "task" {
		return fmt.Errorf("task can not remove")
	}
	if t.DB == nil {
		return fmt.Errorf("db is nil")
	}
	task := &Task{}
	err := t.DB.Where("id = ?", key).First(task).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	task.EntryID = 0
	task.CheckedAt = sql.NullTime{}
	return t.DB.Updates(task).Error
}

func (t *TaskStorage) ListKeys() ([]string, error) {
	if t.DB == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var tasks []*Task
	err := t.DB.Where("status = ?", enum.Enabled).Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(tasks))
	for i := range tasks {
		keys[i] = tasks[i].ID
	}
	keys = append(keys, "task")
	return keys, nil
}
