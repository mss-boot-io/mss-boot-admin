import { getRequestErrorMessage } from '@mss-admin-core/shared/api/errors';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl } from '@umijs/max';
import { Alert, App, Button, Card, Checkbox, Form, Input, Modal, Space, Table, Tag } from 'antd';
import { useState } from 'react';
import { availableManagementActions } from './business-reference';
import type { ManagementCommand, ManagementResolveRequest } from './contract';
import { formatDateTime, formatUsdMicro } from './contract';
import { operationsAPI } from './operations-api';
import { usePendingManagementCommands } from './operations-query';
import { OpsQueryState } from './ui';

interface ResolveValues {
  reason: string;
  confirmed: boolean;
}

export function ManagementRecoveryPanel({ enabled }: { enabled: boolean }) {
  const intl = useIntl();
  const t = (id: string) => intl.formatMessage({ id });
  const { message } = App.useApp();
  const client = useQueryClient();
  const commands = usePendingManagementCommands(enabled);
  const [resolution, setResolution] = useState<{
    command: ManagementCommand;
    value: ManagementResolveRequest['resolution'];
  }>();
  const [form] = Form.useForm<ResolveValues>();
  const refresh = () => client.invalidateQueries({ queryKey: ['litellmops'] });
  const reconcile = useMutation({
    mutationFn: operationsAPI.management.reconcile,
    onSuccess: (command) => message.info(t(`litellmops.management.status.${command.status}`)),
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });
  const resolve = useMutation({
    mutationFn: ({ command, values }: { command: ManagementCommand; values: ResolveValues }) =>
      operationsAPI.management.resolve(command.id, {
        resolution: resolution?.value ?? 'not_applied',
        reason: values.reason.trim(),
        confirm_authoritative_state: true,
      }),
    onSuccess: () => {
      message.success(t('litellmops.management.resolved'));
      setResolution(undefined);
      form.resetFields();
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
    onSettled: refresh,
  });

  if (!enabled) return null;

  const expected = (command: ManagementCommand) => {
    const values = [command.expected.kind, command.expected.affected_fields.join(', ')];
    if (command.expected.max_budget_usd_micro != null)
      values.push(formatUsdMicro(command.expected.max_budget_usd_micro));
    if (command.expected.spend_usd_micro != null)
      values.push(formatUsdMicro(command.expected.spend_usd_micro));
    if (command.expected.blocked != null)
      values.push(
        command.expected.blocked ? t('litellmops.common.blocked') : t('litellmops.common.active'),
      );
    if (command.expected.deleted) values.push(t('litellmops.common.deleted'));
    return values.filter(Boolean).join(' · ') || '—';
  };

  return (
    <>
      <Card
        size="small"
        title={t('litellmops.management.title')}
        extra={
          <Button loading={commands.isFetching} onClick={() => void commands.refetch()}>
            {t('litellmops.common.refresh')}
          </Button>
        }
      >
        <Alert
          showIcon
          type="warning"
          message={t('litellmops.management.warning')}
          style={{ marginBottom: 12 }}
        />
        <OpsQueryState
          data={commands.data}
          error={commands.error}
          isError={commands.isError}
          isPending={commands.isPending}
          onRetry={() => void commands.refetch()}
        >
          <Table<ManagementCommand>
            rowKey="id"
            size="small"
            dataSource={commands.data ?? []}
            scroll={{ x: 1_180 }}
            pagination={false}
            columns={[
              {
                title: t('litellmops.common.createdAt'),
                dataIndex: 'created_at',
                width: 170,
                render: formatDateTime,
              },
              {
                title: t('litellmops.management.user'),
                dataIndex: 'user_email',
                width: 210,
                render: (value) => value || '—',
              },
              {
                title: t('litellmops.management.operation'),
                width: 190,
                render: (_, row) => `${row.target_type} · ${row.action}`,
              },
              {
                title: t('litellmops.management.expected'),
                width: 260,
                render: (_, row) => expected(row),
              },
              {
                title: t('litellmops.common.status'),
                dataIndex: 'status',
                width: 170,
                render: (value) => <Tag>{t(`litellmops.management.status.${value}`)}</Tag>,
              },
              {
                title: t('litellmops.common.error'),
                dataIndex: 'last_error_code',
                width: 170,
                render: (value) => value || '—',
              },
              {
                title: t('litellmops.common.actions'),
                fixed: 'right',
                width: 250,
                render: (_, command) => {
                  const actions = availableManagementActions(command);
                  return (
                    <Space size={2} wrap>
                      {actions.includes('reconcile') ? (
                        <Button
                          size="small"
                          loading={reconcile.isPending && reconcile.variables === command.id}
                          onClick={() => reconcile.mutate(command.id)}
                        >
                          {t('litellmops.management.reconcile')}
                        </Button>
                      ) : null}
                      {actions.includes('resolve_applied') ? (
                        <Button
                          size="small"
                          danger
                          onClick={() => setResolution({ command, value: 'applied' })}
                        >
                          {t('litellmops.management.applied')}
                        </Button>
                      ) : null}
                      {actions.includes('resolve_not_applied') ? (
                        <Button
                          size="small"
                          danger
                          onClick={() => setResolution({ command, value: 'not_applied' })}
                        >
                          {t('litellmops.management.notApplied')}
                        </Button>
                      ) : null}
                    </Space>
                  );
                },
              },
            ]}
          />
        </OpsQueryState>
      </Card>
      <Modal
        destroyOnHidden
        title={t('litellmops.management.manualTitle')}
        open={Boolean(resolution)}
        confirmLoading={resolve.isPending}
        okButtonProps={{ danger: true }}
        onCancel={() => setResolution(undefined)}
        onOk={() =>
          void form
            .validateFields()
            .then((values) => resolution && resolve.mutate({ command: resolution.command, values }))
        }
      >
        <Alert
          showIcon
          type="error"
          message={t('litellmops.management.manualWarning')}
          action={
            <Button href="https://litellm-admin.flypool.io/ui/" target="_blank" rel="noreferrer">
              {t('litellmops.management.openNative')}
            </Button>
          }
        />
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="reason"
            label={t('litellmops.common.reason')}
            rules={[{ required: true, min: 4, max: 512 }]}
          >
            <Input.TextArea rows={3} maxLength={512} />
          </Form.Item>
          <Form.Item
            name="confirmed"
            valuePropName="checked"
            rules={[
              {
                validator: (_, value) =>
                  value
                    ? Promise.resolve()
                    : Promise.reject(new Error(t('litellmops.management.confirmRequired'))),
              },
            ]}
          >
            <Checkbox>{t('litellmops.management.confirmAuthoritative')}</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
