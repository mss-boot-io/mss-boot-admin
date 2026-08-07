package models

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/task"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mss-boot-io/mss-boot-admin/admin/config"
	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 16:11:48
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 16:11:48
 */

var TaskFuncMap = make(map[string]pkg.TaskFunc)

var withTaskDatabase = func(operation func(*gorm.DB) error) error {
	return config.Cfg.WithDatabase(operation)
}

type TaskProvider string

const (
	TaskProviderDefault TaskProvider = "default"
	TaskProviderK8S     TaskProvider = "k8s"
	TaskProviderFunc    TaskProvider = "func"
)

func init() {
	TaskFuncMap["test"] = func(ctx context.Context, args ...string) error {
		fmt.Println("task", time.Now(), args)
		return nil
	}
}

// Task support http/grpc/script
type Task struct {
	ModelGormTenant
	ModelCreator
	Name       string         `json:"name" gorm:"type:varchar(255);not null;comment:任务名称"`
	Namespace  string         `json:"namespace" gorm:"type:varchar(255);not null;comment:命名空间"`
	Cluster    string         `json:"cluster" gorm:"type:varchar(50);comment:集群"`
	Provider   TaskProvider   `json:"provider" gorm:"type:varchar(255);not null;comment:提供者"`
	Image      string         `json:"image" gorm:"type:varchar(255);not null;default:default;comment:镜像"`
	EntryID    int            `json:"entryID" gorm:"size:10;comment:任务ID"`
	Spec       string         `json:"spec" gorm:"type:varchar(255);not null;comment:任务规则"`
	Command    JsonRawMessage `json:"command" gorm:"type:varchar(255);not null;comment:命令"`
	Args       JsonRawMessage `json:"args" gorm:"type:text"`
	Once       bool           `json:"once" gorm:"-"`
	Protocol   string         `json:"protocol" gorm:"size:10"`
	Endpoint   string         `json:"endpoint" gorm:"type:varchar(255);not null;comment:地址"`
	Body       string         `json:"body" gorm:"type:bytes"`
	Status     enum.Status    `json:"status" gorm:"size:10"`
	Remark     string         `json:"remark" gorm:"type:text"`
	CheckedAtR *time.Time     `gorm:"-" json:"checkedAt"`
	CheckedAt  sql.NullTime   `gorm:"index" swaggertype:"string" json:"-"`
	Timeout    int            `json:"timeout"`
	Method     string         `gorm:"size:10" json:"method"`
	Python     string         `json:"python"`
	Metadata   string         `json:"metadata" gorm:"type:bytes"`
}

func (t *Task) GetArgs() []string {
	if len(t.Args) == 0 || t.Args == "[]" {
		return nil
	}
	args := make([]string, 0)
	err := json.Unmarshal([]byte(t.Args), &args)
	if err != nil {
		slog.Error("task args unmarshal error", slog.Any("err", err))
	}
	return args
}

func (t *Task) GetCommand() []string {
	if len(t.Command) == 0 || t.Command == "[]" {
		return nil
	}
	commands := make([]string, 0)
	err := json.Unmarshal([]byte(t.Command), &commands)
	if err != nil {
		slog.Error("task command unmarshal error", slog.Any("err", err))
	}
	return commands
}

func (*Task) TableName() string {
	return "mss_boot_tasks"
}

func (t *Task) AfterCreate(tx *gorm.DB) error {
	switch t.Provider {
	case TaskProviderDefault, "":
		return nil
	case TaskProviderFunc:
		if _, exist := TaskFuncMap[t.Method]; !exist {
			return fmt.Errorf("task func %s does not exist", t.Method)
		}
		return nil
	}
	clientSet := config.Cfg.Clusters.GetClientSet(t.Cluster)
	if clientSet == nil {
		return fmt.Errorf("cluster %s not found", t.Cluster)
	}
	var limitCount int32 = 10
	if t.Namespace == "" {
		t.Namespace = "default"
	}
	// 失败的pod只能重试3次
	_, err := clientSet.BatchV1().CronJobs(t.Namespace).Create(tx.Statement.Context,
		&batchv1.CronJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      t.ID,
				Namespace: t.Namespace,
				Labels: map[string]string{
					"id":   t.ID,
					"name": t.Name,
				},
			},
			Spec: batchv1.CronJobSpec{
				SuccessfulJobsHistoryLimit: &limitCount,
				FailedJobsHistoryLimit:     &limitCount,
				Schedule:                   t.Spec,
				JobTemplate: batchv1.JobTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"id":   t.ID,
							"name": t.Name,
						},
					},
					Spec: batchv1.JobSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:            t.Name,
										Image:           t.Image,
										Command:         t.GetCommand(),
										Args:            t.GetArgs(),
										ImagePullPolicy: corev1.PullIfNotPresent,
										Env: []corev1.EnvVar{
											{
												Name: "STAGE",
												ValueFrom: &corev1.EnvVarSource{
													FieldRef: &corev1.ObjectFieldSelector{
														FieldPath: "metadata.namespace",
													},
												},
											},
										},
									},
								},
								RestartPolicy: corev1.RestartPolicyOnFailure,
							},
						},
					},
				},
			},
		},
		metav1.CreateOptions{})
	if err != nil {
		slog.Error("task create cron job error", slog.Any("err", err))
		return err
	}
	return nil
}

func (t *Task) AfterUpdate(tx *gorm.DB) error {
	switch t.Provider {
	case TaskProviderDefault, "":
		return nil
	case TaskProviderFunc:
		if _, exist := TaskFuncMap[t.Method]; !exist {
			return fmt.Errorf("task func %s not exist", t.Method)
		}
		return nil
	}
	clientSet := config.Cfg.Clusters.GetClientSet(t.Cluster)
	if clientSet == nil {
		return fmt.Errorf("cluster %s not found", t.Cluster)
	}
	if t.Namespace == "" {
		t.Namespace = "default"
	}
	job, err := clientSet.BatchV1().CronJobs(t.Namespace).
		Get(tx.Statement.Context, t.ID, metav1.GetOptions{})
	if err != nil {
		slog.Error("task get cron job error", slog.Any("err", err))
		return err
	}
	job.Spec.Schedule = t.Spec
	job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image = t.Image
	job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command = t.GetCommand()
	job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args = t.GetArgs()
	_, err = clientSet.BatchV1().
		CronJobs(t.Namespace).
		Update(tx.Statement.Context, job, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("task update cron job error", slog.Any("err", err))
		return err
	}
	return nil
}

func (t *Task) AfterDelete(tx *gorm.DB) error {
	switch t.Provider {
	case TaskProviderDefault, "", TaskProviderFunc:
		return nil
	}
	clientSet := config.Cfg.Clusters.GetClientSet(t.Cluster)
	if clientSet == nil {
		return fmt.Errorf("cluster %s not found", t.Cluster)
	}
	return clientSet.BatchV1().CronJobs(t.Namespace).Delete(tx.Statement.Context, t.ID, metav1.DeleteOptions{})
}

func (t *Task) AfterFind(_ *gorm.DB) (err error) {
	if t.CheckedAt.Valid {
		t.CheckedAtR = &t.CheckedAt.Time
	}
	return
}

func (t *Task) BeforeDelete(_ *gorm.DB) (err error) {
	err = task.RemoveJob(t.ID)
	if errors.Is(err, task.ErrUserSchedulesDisabled) {
		return nil
	}
	return err
}

// Run task
func (t *Task) Run() {
	if err := withTaskDatabase(func(db *gorm.DB) error {
		t.runWithDatabase(db)
		return nil
	}); err != nil {
		slog.Error("task run acquire database lease error", slog.Any("err", err))
	}
}

func (t *Task) runWithDatabase(db *gorm.DB) {
	checkedAt := time.Now()
	err := db.Model(&Task{}).
		Where("id = ?", t.ID).
		Where("provider in ?", []TaskProvider{TaskProviderDefault, TaskProviderFunc}).
		Update("checked_at", checkedAt).Error
	if err != nil {
		slog.Error("task run update task error", slog.Any("err", err))
		return
	}

	runTask := &Task{}
	if err := db.Where("id = ?", t.ID).First(runTask).Error; err != nil {
		slog.Error("task run load task error", slog.Any("err", err))
		return
	}
	if runTask.Status != enum.Enabled {
		return
	}

	taskRun := &TaskRun{
		TaskID:   runTask.ID,
		Status:   enum.Locked,
		database: db,
	}
	var command string
	if runTask.GetCommand() != nil && len(runTask.GetCommand()) > 0 {
		command = runTask.GetCommand()[0]
	}
	taskO := &pkg.Task{
		Context:  pkg.WithTaskDatabase(context.Background(), db),
		ID:       runTask.ID,
		Name:     runTask.Name,
		Endpoint: fmt.Sprintf("%s://%s", runTask.Protocol, runTask.Endpoint),
		Method:   runTask.Method,
		Command:  command,
		Body:     bytes.NewBuffer([]byte(runTask.Body)),
		Args:     runTask.GetArgs(),
		Python:   runTask.Python,
		Writer:   taskRun,
		Metadata: make(map[string]string),
		Timeout:  time.Duration(runTask.Timeout) * time.Second,
	}
	if runTask.Provider == TaskProviderFunc {
		taskO.Func, _ = TaskFuncMap[runTask.Method]
	}
	if runTask.Metadata != "" {
		err := json.Unmarshal([]byte(runTask.Metadata), &taskO.Metadata)
		if err != nil {
			slog.Error("task metadata unmarshal error", slog.Any("err", err))
		}
	}
	err = db.Create(taskRun).Error
	if err != nil {
		slog.Error("task run create task run error", slog.Any("err", err))
		return
	}
	err = taskO.Run()
	taskRun.Status = enum.Enabled
	if err != nil {
		slog.Error("task run error", slog.Any("err", err))
		taskRun.Status = enum.Disabled
	}
	err = db.Updates(taskRun).Error
	if err != nil {
		slog.Error("task run update task run error", slog.Any("err", err))
	}
}

func TaskOnce(id string) error {
	return withTaskDatabase(func(db *gorm.DB) error {
		t := &Task{}
		if err := db.Model(&Task{}).Where("id = ?", id).First(t).Error; err != nil {
			return err
		}
		t.runWithDatabase(db)
		return nil
	})
}

type TaskStorage struct {
	// DB remains available for compatibility with callers that own a stable
	// handle. The Admin server uses UseDatabase so configuration reloads cannot
	// leave the scheduler bound to a closed pool.
	DB          *gorm.DB
	UseDatabase func(func(*gorm.DB) error) error
}

func (t *TaskStorage) Get(key string) (entryID cron.EntryID, spec string, job cron.Job, exist bool, err error) {
	err = t.withDatabase(func(db *gorm.DB) error {
		tk := &Task{}
		if queryErr := db.Where("id = ?", key).Not("provider = ?", TaskProviderK8S).First(tk).Error; queryErr != nil {
			if errors.Is(queryErr, gorm.ErrRecordNotFound) {
				return nil
			}
			return queryErr
		}
		entryID = cron.EntryID(tk.EntryID)
		spec = tk.Spec
		job = tk
		exist = true
		return nil
	})
	return
}

func (t *TaskStorage) Set(key string, entryID cron.EntryID, spec string, job cron.Job) error {
	return t.withDatabase(func(db *gorm.DB) error {
		tk := &Task{}
		if err := db.Where("id = ?", key).Where("provider = ?", TaskProviderDefault).First(tk).Error; err != nil {
			return err
		}
		tk.EntryID = int(entryID)
		tk.Spec = spec
		return db.Updates(tk).Error
	})
}

func (t *TaskStorage) Update(key string, entryID cron.EntryID) error {
	return t.withDatabase(func(db *gorm.DB) error {
		tk := &Task{}
		if err := db.Where("id = ?", key).Not("provider = ?", TaskProviderK8S).First(tk).Error; err != nil {
			return err
		}
		tk.EntryID = int(entryID)
		return db.Model(tk).Update("entry_id", entryID).Error
	})
}

func (t *TaskStorage) Remove(key string) error {
	return t.withDatabase(func(db *gorm.DB) error {
		tk := &Task{}
		err := db.Where("id = ?", key).First(tk).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		tk.EntryID = 0
		tk.CheckedAt = sql.NullTime{}
		return db.Model(tk).UpdateColumns(map[string]interface{}{
			"entry_id":   0,
			"checked_at": nil,
		}).Error
	})
}

func (t *TaskStorage) ListKeys() ([]string, error) {
	var tasks []*Task
	if err := t.withDatabase(func(db *gorm.DB) error {
		return db.Where("status = ?", enum.Enabled).Not("provider = ?", TaskProviderK8S).Find(&tasks).Error
	}); err != nil {
		return nil, err
	}
	keys := make([]string, len(tasks))
	for i := range tasks {
		keys[i] = tasks[i].ID
	}
	return keys, nil
}

func (t *TaskStorage) withDatabase(operation func(*gorm.DB) error) error {
	if t == nil {
		return errors.New("task storage is nil")
	}
	if t.UseDatabase != nil {
		return t.UseDatabase(operation)
	}
	if t.DB == nil {
		return errors.New("task database is nil")
	}
	return operation(t.DB)
}
