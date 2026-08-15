import { ReloadOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { Alert, Button, Descriptions, Drawer, Listy, Space, Tag, Typography } from 'antd';
import { getRequestErrorMessage, getRequestStatus } from '@/shared/api/errors';
import { PageEmpty, PageError, PageForbidden, PageLoading } from '@/shared/design-system/PageState';
import { useLanguage } from './query';

interface LanguageDetailDrawerProps {
  id?: string;
  open: boolean;
  onClose: () => void;
}

export default function LanguageDetailDrawer({ id, open, onClose }: LanguageDetailDrawerProps) {
  const intl = useIntl();
  const detail = useLanguage(open ? id : undefined);
  const status = getRequestStatus(detail.error);
  const language = detail.data;

  const content = (() => {
    if (detail.isPending && !language) return <PageLoading rows={6} />;
    if (status === 403) {
      return <PageForbidden message={intl.formatMessage({ id: 'language.forbidden.read' })} />;
    }
    if (detail.isError && !language) {
      return (
        <PageError
          message={getRequestErrorMessage(detail.error)}
          onRetry={() => void detail.refetch()}
          retryLabel={intl.formatMessage({ id: 'actions.retry' })}
          title={intl.formatMessage({ id: 'states.loadError' })}
        />
      );
    }
    if (!language) return null;

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
            title={intl.formatMessage({ id: 'language.detail.refreshFailed' })}
            type="warning"
          />
        ) : null}
        <Descriptions
          bordered
          column={1}
          items={[
            {
              key: 'name',
              label: intl.formatMessage({ id: 'language.field.name' }),
              children: language.name,
            },
            {
              key: 'status',
              label: intl.formatMessage({ id: 'language.field.status' }),
              children: (
                <Tag color={language.status === 'enabled' ? 'green' : 'red'}>
                  {intl.formatMessage({ id: `language.status.${language.status}` })}
                </Tag>
              ),
            },
            {
              key: 'remark',
              label: intl.formatMessage({ id: 'language.field.remark' }),
              children: language.remark || '—',
            },
          ]}
          size="small"
        />
        <Typography.Title level={5} className="mb-0">
          {intl.formatMessage(
            { id: 'language.definitions.title' },
            { count: language.defines.length },
          )}
        </Typography.Title>
        {language.defines.length === 0 ? (
          <PageEmpty description={intl.formatMessage({ id: 'language.definitions.empty' })} />
        ) : (
          <Listy
            height={language.defines.length > 20 ? 480 : undefined}
            items={language.defines}
            rowKey="id"
            virtual={language.defines.length > 20}
            itemRender={(definition) => (
              <Typography.Text>
                <Typography.Text code>{`${definition.group}.${definition.key}`}</Typography.Text>
                {' · '}
                {definition.value || '—'}
              </Typography.Text>
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
      title={language?.name ?? intl.formatMessage({ id: 'language.detail.title' })}
      onClose={onClose}
    >
      {content}
    </Drawer>
  );
}
