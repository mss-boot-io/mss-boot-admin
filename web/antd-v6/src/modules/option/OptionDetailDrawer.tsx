import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import { useIntl } from '@umijs/max';
import { Alert, Button, Descriptions, Drawer, Listy, Space, Tag, Typography } from 'antd';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { presentOptionDetail } from './presentation';
import { useOption } from './query';

interface OptionDetailDrawerProps {
  id?: string;
  open: boolean;
  onClose: () => void;
}

export default function OptionDetailDrawer({ id, open, onClose }: OptionDetailDrawerProps) {
  const intl = useIntl();
  const detail = useOption(open ? id : undefined);
  const status = getRequestStatus(detail.error);
  const option = detail.data;
  const presentedOption = option ? presentOptionDetail(option, intl) : undefined;

  const content = (() => {
    if (detail.isPending && !option) return <PageLoading rows={7} />;
    if (status === 403) {
      return <PageForbidden message={intl.formatMessage({ id: 'option.forbidden.read' })} />;
    }
    if (detail.isError && !option) {
      return (
        <PageError
          message={getRequestErrorMessage(detail.error)}
          onRetry={() => void detail.refetch()}
          retryLabel={intl.formatMessage({ id: 'actions.retry' })}
          title={intl.formatMessage({ id: 'states.loadError' })}
        />
      );
    }
    if (!presentedOption) return null;

    return (
      <Space orientation="vertical" size="middle" className="w-full">
        {detail.isError ? (
          <Alert
            action={
              <Button icon={<ReloadOutlined />} size="small" onClick={() => void detail.refetch()}>
                {intl.formatMessage({ id: 'actions.retry' })}
              </Button>
            }
            showIcon
            title={intl.formatMessage({ id: 'option.detail.refreshFailed' })}
            type="warning"
          />
        ) : null}
        <Descriptions
          bordered
          column={1}
          items={[
            {
              key: 'name',
              label: intl.formatMessage({ id: 'option.field.name' }),
              children: <Typography.Text code>{presentedOption.name}</Typography.Text>,
            },
            {
              key: 'category',
              label: intl.formatMessage({ id: 'option.field.category' }),
              children: <Typography.Text code>{presentedOption.category}</Typography.Text>,
            },
            {
              key: 'displayName',
              label: intl.formatMessage({ id: 'option.field.displayName' }),
              children: presentedOption.displayName || '—',
            },
            {
              key: 'status',
              label: intl.formatMessage({ id: 'option.field.status' }),
              children: (
                <Space wrap>
                  <Tag color={presentedOption.status === 'enabled' ? 'green' : 'red'}>
                    {intl.formatMessage({ id: `option.status.${presentedOption.status}` })}
                  </Tag>
                  {presentedOption.builtIn ? (
                    <Tag>{intl.formatMessage({ id: 'option.builtIn' })}</Tag>
                  ) : null}
                </Space>
              ),
            },
            {
              key: 'version',
              label: intl.formatMessage({ id: 'option.field.version' }),
              children: presentedOption.version,
            },
            {
              key: 'description',
              label: intl.formatMessage({ id: 'option.field.description' }),
              children: presentedOption.description || '—',
            },
            {
              key: 'remark',
              label: intl.formatMessage({ id: 'option.field.remark' }),
              children: presentedOption.remark || '—',
            },
          ]}
          size="small"
        />
        <Typography.Title level={5} className="mb-0">
          {intl.formatMessage(
            { id: 'option.items.title' },
            { count: presentedOption.items.length },
          )}
        </Typography.Title>
        {presentedOption.items.length === 0 ? (
          <PageEmpty description={intl.formatMessage({ id: 'option.items.empty' })} />
        ) : (
          <Listy
            height={presentedOption.items.length > 20 ? 520 : undefined}
            items={presentedOption.items}
            rowKey="id"
            virtual={presentedOption.items.length > 20}
            itemRender={(item) => (
              <Space orientation="vertical" size={2} className="w-full">
                <Typography.Text strong>{item.label}</Typography.Text>
                <Typography.Text>
                  <Typography.Text code>{item.key}</Typography.Text>
                  {' → '}
                  <Typography.Text code>{item.value}</Typography.Text>
                  {` · ${intl.formatMessage({ id: 'option.item.sort' })}: ${item.sort}`}
                </Typography.Text>
                {item.color || item.icon ? (
                  <Typography.Text type="secondary">
                    {item.color
                      ? `${intl.formatMessage({ id: 'option.item.color' })}: ${item.color}`
                      : ''}
                    {item.color && item.icon ? ' · ' : ''}
                    {item.icon
                      ? `${intl.formatMessage({ id: 'option.item.icon' })}: ${item.icon}`
                      : ''}
                  </Typography.Text>
                ) : null}
                {item.extra ? (
                  <Typography.Text type="secondary" code>
                    {JSON.stringify(item.extra)}
                  </Typography.Text>
                ) : null}
              </Space>
            )}
          />
        )}
      </Space>
    );
  })();

  return (
    <Drawer
      destroyOnHidden
      open={open}
      size="large"
      title={
        presentedOption?.displayName ||
        presentedOption?.name ||
        intl.formatMessage({ id: 'option.detail.title' })
      }
      onClose={onClose}
    >
      {content}
    </Drawer>
  );
}
