import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageContainer } from '@mss-admin-core/shared/design-system/PageContainer';
import { PageForbidden } from '@mss-admin-core/shared/design-system/PageState';
import { useIntl, useModel } from '@umijs/max';
import { Alert, Button, Card, Descriptions, Drawer, Empty, Space, Table, Tag } from 'antd';
import { useMemo, useState } from 'react';
import { buildOpsAccess } from '@/modules/litellmops/permissions';
import { useOrgDetail, useOrgList } from '@/modules/litellmops/query';
import { formatUsd, type RemoteOrg, type RemoteTeam } from '@/modules/litellmops/types';
import { OpsQueryState } from '@/modules/litellmops/ui';

const nativeLiteLLMURL = 'https://litellm-admin.flypool.io/ui/';

export default function LitellmOpsOrgsPage() {
  const intl = useIntl();
  const t = (id: string) => intl.formatMessage({ id });
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const access = useMemo(
    () => buildOpsAccess(initialState?.currentUser),
    [initialState?.currentUser],
  );
  const organizations = useOrgList();
  const [detailId, setDetailId] = useState<string>();

  if (!access.canReadOrganizations) return <PageForbidden />;

  return (
    <PageContainer title={t('litellmops.orgs.title')} content={t('litellmops.orgs.description')}>
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="warning"
          showIcon
          message={t('litellmops.orgs.readOnly')}
          action={
            <Button href={nativeLiteLLMURL} target="_blank" rel="noreferrer">
              {t('litellmops.management.openNative')}
            </Button>
          }
        />
        <Card
          title={t('litellmops.orgs.organizations')}
          extra={
            <Button onClick={() => void organizations.refetch()}>
              {t('litellmops.common.refresh')}
            </Button>
          }
        >
          <OpsQueryState
            data={organizations.data}
            error={organizations.error}
            isError={organizations.isError}
            isPending={organizations.isPending}
            onRetry={() => void organizations.refetch()}
          >
            <Table<RemoteOrg>
              rowKey="organization_id"
              loading={organizations.isFetching}
              dataSource={organizations.data?.items ?? []}
              locale={{ emptyText: <Empty description={t('litellmops.orgs.empty')} /> }}
              columns={[
                {
                  title: t('litellmops.orgs.name'),
                  dataIndex: 'organization_alias',
                  render: (value) => value || '—',
                },
                {
                  title: t('litellmops.orgs.id'),
                  dataIndex: 'organization_id',
                  render: (value: string) => <code>{value}</code>,
                },
                {
                  title: t('litellmops.users.budget'),
                  dataIndex: 'max_budget',
                  render: (value: number | null) => formatUsd(value, 2),
                },
                {
                  title: t('litellmops.users.spend'),
                  dataIndex: 'spend',
                  render: (value: number) => formatUsd(value),
                },
                {
                  title: t('litellmops.orgs.members'),
                  render: (_: unknown, record) => record.members?.length ?? 0,
                },
                {
                  title: t('litellmops.orgs.teams'),
                  render: (_: unknown, record) => record.teams?.length ?? 0,
                },
                {
                  title: t('litellmops.common.actions'),
                  render: (_: unknown, record) =>
                    access.canReadOrganizationDetails ? (
                      <Button
                        type="link"
                        size="small"
                        onClick={() => setDetailId(record.organization_id)}
                      >
                        {t('litellmops.common.detail')}
                      </Button>
                    ) : null,
                },
              ]}
              pagination={false}
              scroll={{ x: 900 }}
            />
          </OpsQueryState>
        </Card>
        <OrgDetailDrawer id={detailId} onClose={() => setDetailId(undefined)} />
      </Space>
    </PageContainer>
  );
}

function OrgDetailDrawer({ id, onClose }: { id?: string; onClose: () => void }) {
  const intl = useIntl();
  const t = (messageId: string) => intl.formatMessage({ id: messageId });
  const organization = useOrgDetail(id);
  const data = organization.data;
  return (
    <Drawer
      destroyOnHidden
      title={data?.organization_alias || t('litellmops.orgs.detail')}
      width="min(860px, 100vw)"
      open={Boolean(id)}
      onClose={onClose}
    >
      <OpsQueryState
        data={data}
        error={organization.error}
        isError={organization.isError}
        isPending={organization.isPending}
        onRetry={() => void organization.refetch()}
      >
        {data ? (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions size="small" bordered column={{ xs: 1, sm: 2 }}>
              <Descriptions.Item label={t('litellmops.orgs.id')}>
                <code>{data.organization_id}</code>
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.budget')}>
                {formatUsd(data.max_budget, 2)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.users.spend')}>
                {formatUsd(data.spend)}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.orgs.duration')}>
                {data.budget_duration || '—'}
              </Descriptions.Item>
              <Descriptions.Item label={t('litellmops.orgs.models')} span={2}>
                {(data.models ?? []).length === 0
                  ? '—'
                  : data.models.map((model) => <Tag key={model}>{model}</Tag>)}
              </Descriptions.Item>
            </Descriptions>

            <Card size="small" title={t('litellmops.orgs.members')}>
              <Table
                rowKey={(row) => row.user_id || row.user_email}
                size="small"
                pagination={false}
                dataSource={data.members ?? []}
                locale={{ emptyText: <Empty description={t('litellmops.orgs.noMembers')} /> }}
                columns={[
                  {
                    title: t('litellmops.users.email'),
                    dataIndex: 'user_email',
                    render: (value) => value || '—',
                  },
                  {
                    title: t('litellmops.users.role'),
                    dataIndex: 'role',
                    render: (value: string) => <Tag>{value || '—'}</Tag>,
                  },
                  {
                    title: t('litellmops.orgs.memberBudget'),
                    dataIndex: 'max_budget_in_organization',
                    render: (value: number | null) => formatUsd(value, 2),
                  },
                ]}
              />
            </Card>

            <Card size="small" title={t('litellmops.orgs.teams')}>
              <Table<RemoteTeam>
                rowKey="team_id"
                size="small"
                pagination={false}
                dataSource={data.teams ?? []}
                locale={{ emptyText: <Empty description={t('litellmops.orgs.noTeams')} /> }}
                columns={[
                  {
                    title: t('litellmops.orgs.name'),
                    dataIndex: 'team_alias',
                    render: (value) => value || '—',
                  },
                  {
                    title: t('litellmops.users.budget'),
                    dataIndex: 'max_budget',
                    render: (value: number | null) => formatUsd(value, 2),
                  },
                  {
                    title: t('litellmops.users.spend'),
                    dataIndex: 'spend',
                    render: (value: number) => formatUsd(value),
                  },
                ]}
              />
            </Card>
          </Space>
        ) : null}
      </OpsQueryState>
    </Drawer>
  );
}
