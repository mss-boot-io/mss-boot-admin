import CopyOutlined from '@ant-design/icons/CopyOutlined';
import DeleteOutlined from '@ant-design/icons/DeleteOutlined';
import PlusOutlined from '@ant-design/icons/PlusOutlined';
import ReloadOutlined from '@ant-design/icons/ReloadOutlined';
import SafetyCertificateOutlined from '@ant-design/icons/SafetyCertificateOutlined';
import SyncOutlined from '@ant-design/icons/SyncOutlined';
import { getRequestErrorMessage } from '@mss-admin-core/shared/api/client';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { PageError, PageLoading } from '@mss-admin-core/shared/design-system/PageState';
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { useQuery } from '@tanstack/react-query';
import { useIntl, useModel } from '@umijs/max';
import {
  Alert,
  App,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Tag,
  Typography,
} from 'antd';
import { useState } from 'react';
import { accountAPI } from './api';
import type { AccessTokenSecret } from './contracts';

function formatDate(value: string | undefined, locale: string): string {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '—';
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

export default function AccessTokensPanel() {
  const intl = useIntl();
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const userID = initialState?.currentUser?.id ?? '';
  const [form] = Form.useForm<{ validityPeriod: string }>();
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [rotatingID, setRotatingID] = useState<string>();
  const [revokingID, setRevokingID] = useState<string>();
  const [secret, setSecret] = useState<AccessTokenSecret>();
  const [operationError, setOperationError] = useState<string>();

  const tokens = useQuery({
    queryKey: queryKeys.accountTokens(userID),
    queryFn: accountAPI.listAccessTokens,
    enabled: Boolean(userID),
    staleTime: 0,
  });
  const interactionBlocked = Boolean(creating || rotatingID || revokingID || secret);

  const refreshList = () =>
    queryClient.invalidateQueries({ queryKey: queryKeys.accountTokens(userID) });

  const createToken = async ({ validityPeriod }: { validityPeriod: string }) => {
    if (interactionBlocked) return;
    setCreating(true);
    setOperationError(undefined);
    try {
      // Do not use a React Query mutation for this call: mutation result data
      // is cached. The raw secret belongs only in this component's one-time
      // memory state and is discarded when the modal closes.
      const created = await accountAPI.createAccessToken(validityPeriod);
      setCreateOpen(false);
      form.resetFields();
      setSecret(created);
      void refreshList();
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      setCreating(false);
    }
  };

  const rotateToken = async (id: string) => {
    if (interactionBlocked) return;
    setRotatingID(id);
    setOperationError(undefined);
    try {
      const rotated = await accountAPI.rotateAccessToken(id);
      setSecret(rotated);
      void refreshList();
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      setRotatingID(undefined);
    }
  };

  const revokeToken = async (id: string) => {
    if (interactionBlocked) return;
    setRevokingID(id);
    setOperationError(undefined);
    try {
      await accountAPI.revokeAccessToken(id);
      await refreshList();
      void message.success(intl.formatMessage({ id: 'account.tokens.revoked' }));
    } catch (error) {
      setOperationError(getRequestErrorMessage(error));
    } finally {
      setRevokingID(undefined);
    }
  };

  if (tokens.isPending && !tokens.data) return <PageLoading rows={5} />;
  if (tokens.isError && !tokens.data) {
    return (
      <PageError
        message={intl.formatMessage({ id: 'account.tokens.loadFailed' })}
        onRetry={() => void tokens.refetch()}
        retryLabel={intl.formatMessage({ id: 'actions.retry' })}
      />
    );
  }

  return (
    <Space orientation="vertical" size="large" className="w-full">
      <Alert
        showIcon
        icon={<SafetyCertificateOutlined />}
        type="info"
        title={intl.formatMessage({ id: 'account.tokens.securityTitle' })}
        description={intl.formatMessage({ id: 'account.tokens.securityDescription' })}
      />
      {operationError ? (
        <Alert
          closable
          showIcon
          type="error"
          title={intl.formatMessage({ id: 'account.tokens.operationFailed' })}
          description={operationError}
          onClose={() => setOperationError(undefined)}
        />
      ) : null}
      <div className="flex flex-wrap justify-between gap-3">
        <Typography.Title level={4} className="m-0">
          {intl.formatMessage({ id: 'account.tokens.title' })}
        </Typography.Title>
        <Space wrap>
          <Button icon={<ReloadOutlined />} onClick={() => void tokens.refetch()}>
            {intl.formatMessage({ id: 'actions.refresh' })}
          </Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            disabled={interactionBlocked}
            onClick={() => {
              setOperationError(undefined);
              setCreateOpen(true);
            }}
          >
            {intl.formatMessage({ id: 'account.tokens.create' })}
          </Button>
        </Space>
      </div>
      {!tokens.data?.length ? (
        <Empty description={intl.formatMessage({ id: 'account.tokens.empty' })} />
      ) : (
        <ul className="m-0 list-none space-y-3 p-0">
          {tokens.data.map((token) => (
            <li key={token.id}>
              <Card
                className="w-full"
                size="small"
                title={
                  <Space wrap>
                    <Typography.Text code>{token.id}</Typography.Text>
                    {token.fingerprint ? <Tag>{token.fingerprint}</Tag> : null}
                  </Space>
                }
                extra={
                  <Space wrap>
                    <Popconfirm
                      title={intl.formatMessage({ id: 'account.tokens.rotateConfirm' })}
                      description={intl.formatMessage({ id: 'account.tokens.rotateWarning' })}
                      onConfirm={() => rotateToken(token.id)}
                    >
                      <Button
                        type="text"
                        icon={<SyncOutlined />}
                        loading={rotatingID === token.id}
                        disabled={interactionBlocked}
                      >
                        {intl.formatMessage({ id: 'account.tokens.rotate' })}
                      </Button>
                    </Popconfirm>
                    <Popconfirm
                      title={intl.formatMessage({ id: 'account.tokens.revokeConfirm' })}
                      description={intl.formatMessage({ id: 'account.tokens.revokeWarning' })}
                      okButtonProps={{ danger: true }}
                      onConfirm={() => revokeToken(token.id)}
                    >
                      <Button
                        danger
                        type="text"
                        icon={<DeleteOutlined />}
                        loading={revokingID === token.id}
                        disabled={interactionBlocked}
                      >
                        {intl.formatMessage({ id: 'account.tokens.revoke' })}
                      </Button>
                    </Popconfirm>
                  </Space>
                }
              >
                <Typography.Text type="secondary">
                  {intl.formatMessage(
                    { id: 'account.tokens.expires' },
                    { value: formatDate(token.expiredAt, intl.locale) },
                  )}
                </Typography.Text>
              </Card>
            </li>
          ))}
        </ul>
      )}

      <Modal
        title={intl.formatMessage({ id: 'account.tokens.create' })}
        open={createOpen}
        destroyOnHidden
        forceRender
        confirmLoading={creating}
        onCancel={() => {
          if (!creating) setCreateOpen(false);
        }}
        onOk={() => form.submit()}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ validityPeriod: '8760h' }}
          onFinish={(values) => void createToken(values)}
        >
          <Form.Item
            name="validityPeriod"
            label={intl.formatMessage({ id: 'account.tokens.validity' })}
            rules={[{ required: true }]}
          >
            <Select
              options={[
                { value: '24h', label: intl.formatMessage({ id: 'account.tokens.oneDay' }) },
                { value: '720h', label: intl.formatMessage({ id: 'account.tokens.thirtyDays' }) },
                { value: '8760h', label: intl.formatMessage({ id: 'account.tokens.oneYear' }) },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={intl.formatMessage({ id: 'account.tokens.secretTitle' })}
        open={Boolean(secret)}
        closable={false}
        keyboard={false}
        mask={{ closable: false }}
        destroyOnHidden
        footer={
          <Space wrap>
            <Button
              icon={<CopyOutlined />}
              onClick={async () => {
                if (!secret?.token) return;
                try {
                  await navigator.clipboard.writeText(secret.token);
                  void message.success(intl.formatMessage({ id: 'account.tokens.copied' }));
                } catch {
                  void message.error(intl.formatMessage({ id: 'account.tokens.copyFailed' }));
                }
              }}
            >
              {intl.formatMessage({ id: 'account.tokens.copy' })}
            </Button>
            <Button type="primary" onClick={() => setSecret(undefined)}>
              {intl.formatMessage({ id: 'account.tokens.savedSecret' })}
            </Button>
          </Space>
        }
      >
        <Alert
          className="mb-4"
          showIcon
          type="warning"
          title={intl.formatMessage({ id: 'account.tokens.secretOnce' })}
        />
        <Input.TextArea
          readOnly
          autoSize={{ minRows: 3, maxRows: 6 }}
          value={secret?.token}
          className="font-mono"
        />
      </Modal>
    </Space>
  );
}
