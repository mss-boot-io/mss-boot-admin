import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Empty,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
  type TableColumnsType,
} from 'antd';
import { useMemo } from 'react';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/errors';
import type { GatewayModel } from '../contract';
import { formatDateTime } from '../contract';
import { operationsAPI } from '../operations-api';
import { useGatewayHealth, useGatewayModels } from '../operations-query';
import { buildOpsAccess } from '../permissions';
import { OpsQueryState } from '../ui';

export default function GatewayView() {
  const intl = useIntl();
  const t = (id: string, values?: Record<string, string | number>) => intl.formatMessage({ id }, values);
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(() => buildOpsAccess(initialState?.currentUser), [initialState?.currentUser]);
  const { message, modal } = App.useApp();
  const client = useQueryClient();
  const health = useGatewayHealth();
  const models = useGatewayModels();

  const toggle = useMutation({
    mutationFn: ({ id, blocked }: { id: string; blocked: boolean }) =>
      blocked ? operationsAPI.gateway.unblock(id) : operationsAPI.gateway.block(id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ['litellmops', 'gateway'] });
      message.success(t('litellmops.gateway.modelUpdated'));
    },
    onError: (error) => message.error(getRequestErrorMessage(error)),
  });

  if (!access.canReadGateway) return <PageForbidden />;

  const confirmToggle = (model: GatewayModel) => {
    const action = model.blocked ? 'unblock' : 'block';
    modal.confirm({
      title: t(`litellmops.common.${action}`),
      content: t('litellmops.gateway.toggleWarning', { model: model.name }),
      okText: t(`litellmops.common.${action}`),
      okButtonProps: { danger: !model.blocked },
      onOk: () => toggle.mutateAsync({ id: model.id, blocked: model.blocked }),
    });
  };

  const columns: TableColumnsType<GatewayModel> = [
    {
      title: t('litellmops.gateway.model'),
      dataIndex: 'name',
      fixed: 'left',
      width: 220,
      render: (value) => <code>{value}</code>,
    },
    { title: t('litellmops.gateway.provider'), dataIndex: 'provider', width: 140, render: (value) => value || '—' },
    { title: t('litellmops.gateway.mode'), dataIndex: 'mode', width: 120, render: (value) => value || '—' },
    {
      title: t('litellmops.gateway.health'),
      dataIndex: 'healthy',
      width: 120,
      render: (healthy) => (
        <Tag color={healthy === true ? 'success' : healthy === false ? 'error' : 'default'}>
          {t(healthy === true ? 'litellmops.gateway.healthy' : healthy === false ? 'litellmops.gateway.unhealthy' : 'litellmops.gateway.unknown')}
        </Tag>
      ),
    },
    {
      title: t('litellmops.common.status'),
      dataIndex: 'blocked',
      width: 110,
      render: (blocked) => (
        <Tag color={blocked ? 'error' : 'success'}>
          {t(blocked ? 'litellmops.common.disabled' : 'litellmops.common.enabled')}
        </Tag>
      ),
    },
    { title: t('litellmops.gateway.endpoints'), dataIndex: 'endpoint_count', width: 100, render: (value) => value ?? '—' },
    { title: t('litellmops.gateway.lastChecked'), dataIndex: 'last_checked_at', width: 180, render: formatDateTime },
    {
      title: t('litellmops.common.actions'),
      key: 'actions',
      fixed: 'right',
      width: 100,
      render: (_, model) =>
        access.canWriteGateway ? (
          <Button
            size="small"
            danger={!model.blocked}
            loading={toggle.isPending && toggle.variables?.id === model.id}
            onClick={() => confirmToggle(model)}
          >
            {t(model.blocked ? 'litellmops.common.unblock' : 'litellmops.common.block')}
          </Button>
        ) : null,
    },
  ];

  return (
    <PageContainer title={t('litellmops.gateway.title')} content={t('litellmops.gateway.description')}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          showIcon
          type="info"
          message={t('litellmops.gateway.sensitiveBoundary')}
          action={
            <Space wrap>
              <Button
                href="https://litellm-admin.flypool.io/ui/models-and-endpoints"
                target="_blank"
                rel="noreferrer"
              >
                {t('litellmops.gateway.nativeModels')}
              </Button>
              <Button
                href="https://litellm-admin.flypool.io/ui/router-settings"
                target="_blank"
                rel="noreferrer"
              >
                {t('litellmops.gateway.nativeRouting')}
              </Button>
            </Space>
          }
        />

        <OpsQueryState
          data={health.data}
          error={health.error}
          isError={health.isError}
          isPending={health.isPending}
          onRetry={() => void health.refetch()}
        >
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}>
              <Card size="small">
                <Statistic
                  title={t('litellmops.gateway.readiness')}
                  value={t(`litellmops.gateway.status.${health.data?.status ?? 'unavailable'}`)}
                  valueStyle={{ color: health.data?.ready ? '#389e0d' : '#cf1322' }}
                />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card size="small">
                <Statistic title={t('litellmops.gateway.models')} value={health.data?.model_count ?? models.data?.total ?? 0} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card size="small">
                <Statistic title={t('litellmops.gateway.healthyModels')} value={health.data?.healthy_model_count ?? 0} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card size="small">
                <Statistic title={t('litellmops.gateway.blockedModels')} value={health.data?.blocked_model_count ?? 0} />
              </Card>
            </Col>
          </Row>
          {health.data?.message ? <Alert showIcon type={health.data.ready ? 'success' : 'warning'} message={health.data.message} /> : null}
        </OpsQueryState>

        <Card
          title={t('litellmops.gateway.models')}
          extra={<Button onClick={() => void Promise.all([health.refetch(), models.refetch()])}>{t('litellmops.common.refresh')}</Button>}
        >
          <OpsQueryState
            data={models.data}
            error={models.error}
            isError={models.isError}
            isPending={models.isPending}
            onRetry={() => void models.refetch()}
          >
            <Table<GatewayModel>
              rowKey="id"
              loading={models.isFetching}
              columns={columns}
              dataSource={models.data?.items ?? []}
              locale={{ emptyText: <Empty description={t('litellmops.gateway.empty')} /> }}
              pagination={false}
              scroll={{ x: 1_100 }}
            />
          </OpsQueryState>
        </Card>
      </Space>
    </PageContainer>
  );
}
