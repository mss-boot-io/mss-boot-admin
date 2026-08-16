import EditOutlined from '@ant-design/icons/EditOutlined';
import MailOutlined from '@ant-design/icons/MailOutlined';
import PhoneOutlined from '@ant-design/icons/PhoneOutlined';
import UserOutlined from '@ant-design/icons/UserOutlined';
import { PageContainer, ProCard } from '@ant-design/pro-components';
import { useQuery } from '@tanstack/react-query';
import { Link, useIntl, useModel } from '@umijs/max';
import { Avatar, Button, Descriptions, Space, Tag, Typography } from 'antd';
import { fetchCurrentUser } from '@/shared/auth/session';
import type { InitialState } from '@/shared/auth/types';
import { PageError, PageLoading } from '@/shared/design-system/PageState';
import { queryKeys } from '@/shared/query/client';

function display(value?: string): string {
  return value?.trim() || '—';
}

export default function AccountCenterPage() {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const user = useQuery({
    queryKey: queryKeys.currentUser,
    queryFn: async () => (await fetchCurrentUser()) ?? null,
    initialData: initialState?.currentUser,
  });

  if (user.isPending && !user.data) return <PageLoading rows={7} />;
  if (user.isError || !user.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.center.loadFailed' })}
        onRetry={() => void user.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  const current = user.data;
  const address = [current.country, current.province, current.city, current.address]
    .filter(Boolean)
    .join(' · ');

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.accountCenter.title' })}
      extra={[
        <Link key="settings" to="/account/settings" prefetch>
          <Button type="primary" icon={<EditOutlined />}>
            {intl.formatMessage({ id: 'account.center.edit' })}
          </Button>
        </Link>,
      ]}
    >
      <ProCard gutter={[16, 16]} wrap>
        <ProCard colSpan={{ xs: 24, lg: 7 }} variant="outlined">
          <Space orientation="vertical" align="center" size="middle" className="w-full py-4">
            <Avatar size={112} src={current.avatar || undefined} icon={<UserOutlined />} />
            <div className="text-center">
              <Typography.Title level={3} className="mb-1">
                {display(current.name ?? current.username)}
              </Typography.Title>
              <Typography.Text type="secondary">@{display(current.username)}</Typography.Text>
            </div>
            {current.signature ? (
              <Typography.Paragraph className="text-center" type="secondary">
                {current.signature}
              </Typography.Paragraph>
            ) : null}
            <Space wrap className="justify-center">
              {(current.tags ?? []).map((tag) => (
                <Tag key={tag}>{tag}</Tag>
              ))}
            </Space>
          </Space>
        </ProCard>
        <ProCard colSpan={{ xs: 24, lg: 17 }} variant="outlined">
          <Descriptions
            bordered
            column={{ xs: 1, md: 2 }}
            items={[
              {
                key: 'email',
                label: intl.formatMessage({ id: 'account.profile.email' }),
                children: (
                  <Space>
                    <MailOutlined /> {display(current.email)}
                  </Space>
                ),
              },
              {
                key: 'phone',
                label: intl.formatMessage({ id: 'account.profile.phone' }),
                children: (
                  <Space>
                    <PhoneOutlined /> {display(current.phone)}
                  </Space>
                ),
              },
              {
                key: 'role',
                label: intl.formatMessage({ id: 'account.center.role' }),
                children: display(current.role?.name),
              },
              {
                key: 'department',
                label: intl.formatMessage({ id: 'account.center.department' }),
                children: display(current.department?.name),
              },
              {
                key: 'post',
                label: intl.formatMessage({ id: 'account.center.post' }),
                children: display(current.post?.name),
              },
              {
                key: 'title',
                label: intl.formatMessage({ id: 'account.profile.title' }),
                children: display(current.title),
              },
              {
                key: 'group',
                label: intl.formatMessage({ id: 'account.profile.group' }),
                children: display(current.group),
                span: { xs: 1, md: 2 },
              },
              {
                key: 'address',
                label: intl.formatMessage({ id: 'account.profile.address' }),
                children: display(address),
                span: { xs: 1, md: 2 },
              },
              {
                key: 'profile',
                label: intl.formatMessage({ id: 'account.profile.profile' }),
                children: display(current.profile),
                span: { xs: 1, md: 2 },
              },
            ]}
          />
        </ProCard>
      </ProCard>
    </PageContainer>
  );
}
