package models

import (
	"context"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesTaskLifecycleKeepsCronJobSuspendedStateAligned(t *testing.T) {
	client := fake.NewSimpleClientset()
	previous := taskKubernetesClientFor
	taskKubernetesClientFor = func(cluster string) taskKubernetesClient {
		require.Equal(t, "primary", cluster)
		return client
	}
	t.Cleanup(func() { taskKubernetesClientFor = previous })

	ctx := context.Background()
	tx := &gorm.DB{Statement: &gorm.Statement{Context: ctx}}
	definition := &Task{
		Name:      "cleanup",
		Cluster:   "primary",
		Namespace: "default",
		Provider:  TaskProviderK8S,
		Image:     "example.test/cleanup:1",
		Spec:      "*/5 * * * *",
		Command:   `["cleanup"]`,
		Args:      `[]`,
		Status:    enum.Disabled,
	}
	definition.ID = "cleanup-task"

	require.NoError(t, definition.AfterCreate(tx))
	job, err := client.BatchV1().CronJobs("default").Get(ctx, definition.ID, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, job.Spec.Suspend)
	require.True(t, *job.Spec.Suspend)

	definition.Status = enum.Enabled
	definition.Image = "example.test/cleanup:2"
	require.NoError(t, definition.AfterUpdate(tx))
	job, err = client.BatchV1().CronJobs("default").Get(ctx, definition.ID, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, job.Spec.Suspend)
	require.False(t, *job.Spec.Suspend)
	require.Equal(t, definition.Image, job.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image)

	require.NoError(t, definition.SetKubernetesEnabled(ctx, false))
	job, err = client.BatchV1().CronJobs("default").Get(ctx, definition.ID, metav1.GetOptions{})
	require.NoError(t, err)
	require.True(t, *job.Spec.Suspend)

	require.NoError(t, definition.AfterDelete(tx))
	_, err = client.BatchV1().CronJobs("default").Get(ctx, definition.ID, metav1.GetOptions{})
	require.True(t, errors.IsNotFound(err))
}
