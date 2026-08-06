import React, { useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Skeleton,
  Space,
  Typography,
} from 'antd';
import { useRequest } from 'ahooks';
import { useIntl } from '@umijs/max';
import {
  getUserAuthTokens,
  postUserAuthTokenGenerate,
  putUserAuthTokenIdRevoke,
} from '@/services/admin/userAuthToken';
import { CopyOutlined, DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';

const formatDateTime = (dateString?: string) => {
  if (!dateString) {
    return '-';
  }

  const date = new Date(dateString);
  if (Number.isNaN(date.getTime())) {
    return '-';
  }

  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

type OperationNotice = {
  type: 'success' | 'error';
  message: string;
};

const toTokenSummary = (item: API.UserAuthTokenSummary): API.UserAuthTokenSummary => ({
  id: item.id,
  userID: item.userID,
  fingerprint: item.fingerprint,
  expiredAt: item.expiredAt,
  revoked: item.revoked,
  createdAt: item.createdAt,
  updatedAt: item.updatedAt,
});

const AccessTokenView: React.FC = () => {
  const intl = useIntl();
  const [createForm] = Form.useForm<API.postUserAuthTokenGenerateParams>();
  const [createOpen, setCreateOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string>();
  const [oneTimeSecret, setOneTimeSecret] = useState<string>();
  const [copyStatus, setCopyStatus] = useState<'success' | 'error'>();
  const [copying, setCopying] = useState(false);
  const [revokingID, setRevokingID] = useState<string>();
  const [operationNotice, setOperationNotice] = useState<OperationNotice>();

  const {
    data: accessTokenData,
    loading,
    error,
    refreshAsync: refreshTokens,
  } = useRequest(
    async () => {
      const response = await getUserAuthTokens({ skipErrorHandler: true });
      // Keep the list state metadata-only even during a mixed-version rollout
      // where an older server could still serialize a legacy `token` field.
      return (response.data || []).map(toTokenSummary);
    },
    {
      onError: () => undefined,
    },
  );

  const openCreateDialog = () => {
    createForm.resetFields();
    setCreateError(undefined);
    setCreateOpen(true);
  };

  const closeCreateDialog = () => {
    if (creating) {
      return;
    }
    setCreateOpen(false);
    setCreateError(undefined);
    createForm.resetFields();
  };

  const createToken = async (values: API.postUserAuthTokenGenerateParams) => {
    setCreating(true);
    setCreateError(undefined);
    setOperationNotice(undefined);

    try {
      const response = await postUserAuthTokenGenerate(values, { skipErrorHandler: true });
      if (!response?.token) {
        throw new Error('PAT create response did not contain a token');
      }

      setCreateOpen(false);
      createForm.resetFields();
      setCopyStatus(undefined);
      setOneTimeSecret(response.token);
      void refreshTokens().catch(() => undefined);
    } catch {
      setCreateError(
        intl.formatMessage({
          id: 'pages.accessToken.settings.createFailed',
          defaultMessage: 'Unable to create the access token. Please try again.',
        }),
      );
    } finally {
      setCreating(false);
    }
  };

  const closeOneTimeSecret = () => {
    setOneTimeSecret(undefined);
    setCopyStatus(undefined);
  };

  const copyOneTimeSecret = async () => {
    if (!oneTimeSecret) {
      return;
    }

    setCopying(true);
    setCopyStatus(undefined);
    try {
      await navigator.clipboard.writeText(oneTimeSecret);
      setCopyStatus('success');
    } catch {
      setCopyStatus('error');
    } finally {
      setCopying(false);
    }
  };

  const revokeToken = async (id: string) => {
    setRevokingID(id);
    setOperationNotice(undefined);
    try {
      await putUserAuthTokenIdRevoke({ id }, { skipErrorHandler: true });
      setOperationNotice({
        type: 'success',
        message: intl.formatMessage({
          id: 'pages.accessToken.settings.revokeSuccess',
          defaultMessage: 'Access token revoked.',
        }),
      });
      try {
        await refreshTokens();
      } catch {
        // The list-level error state supplies the retry action. Revocation has
        // already succeeded and must not be reported as failed.
      }
    } catch {
      setOperationNotice({
        type: 'error',
        message: intl.formatMessage({
          id: 'pages.accessToken.settings.revokeFailed',
          defaultMessage: 'Unable to revoke the access token. Please try again.',
        }),
      });
    } finally {
      setRevokingID(undefined);
    }
  };

  const oneYearLater = new Date();
  oneYearLater.setFullYear(oneYearLater.getFullYear() + 1);

  const renderTokenList = () => {
    if (loading && accessTokenData === undefined) {
      return (
        <div
          role="status"
          aria-label={intl.formatMessage({
            id: 'pages.accessToken.settings.loading',
            defaultMessage: 'Loading access tokens',
          })}
        >
          <Skeleton active paragraph={{ rows: 3 }} />
        </div>
      );
    }

    if (error && accessTokenData === undefined) {
      return null;
    }

    if (!accessTokenData || accessTokenData.length === 0) {
      return (
        <Empty
          description={intl.formatMessage({
            id: 'pages.accessToken.settings.noData',
            defaultMessage: 'No access tokens',
          })}
        />
      );
    }

    return (
      <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
        {accessTokenData.map((item) => (
          <Card
            key={item.id}
            type="inner"
            title={`${intl.formatMessage({
              id: 'pages.accessToken.settings.id',
              defaultMessage: 'ID',
            })}: ${item.id}`}
            extra={
              <Popconfirm
                title={intl.formatMessage({
                  id: 'pages.accessToken.settings.revokeConfirmTitle',
                  defaultMessage: 'Revoke this access token?',
                })}
                description={intl.formatMessage({
                  id: 'pages.accessToken.settings.revokeConfirmDescription',
                  defaultMessage: 'Applications using this token will lose access immediately.',
                })}
                okText={intl.formatMessage({
                  id: 'pages.accessToken.settings.revokeConfirmAction',
                  defaultMessage: 'Confirm revoke',
                })}
                cancelText={intl.formatMessage({
                  id: 'pages.title.cancel',
                  defaultMessage: 'Cancel',
                })}
                okButtonProps={{ danger: true, loading: revokingID === item.id }}
                cancelButtonProps={{ disabled: revokingID === item.id }}
                onConfirm={() => revokeToken(item.id)}
              >
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  loading={revokingID === item.id}
                  disabled={Boolean(revokingID)}
                  aria-label={`${intl.formatMessage({
                    id: 'pages.accessToken.settings.revoke',
                    defaultMessage: 'Revoke',
                  })} ${item.id}`}
                >
                  {intl.formatMessage({
                    id:
                      revokingID === item.id
                        ? 'pages.accessToken.settings.revoking'
                        : 'pages.accessToken.settings.revoke',
                    defaultMessage: revokingID === item.id ? 'Revoking' : 'Revoke',
                  })}
                </Button>
              </Popconfirm>
            }
          >
            {item.fingerprint && (
              <div>
                {intl.formatMessage({
                  id: 'pages.accessToken.settings.fingerprint',
                  defaultMessage: 'Fingerprint',
                })}
                : {item.fingerprint}
              </div>
            )}
            <div>
              {intl.formatMessage({
                id: 'pages.accessToken.settings.expired',
                defaultMessage: 'Expires at',
              })}
              :{' '}
              {item.expiredAt && new Date(item.expiredAt).getTime() > oneYearLater.getTime()
                ? intl.formatMessage({
                    id: 'pages.accessToken.settings.longTime',
                    defaultMessage: 'Long term',
                  })
                : formatDateTime(item.expiredAt)}
            </div>
          </Card>
        ))}
      </Space>
    );
  };

  return (
    <>
      <Card
        title={intl.formatMessage({
          id: 'pages.accessToken.settings.title',
          defaultMessage: 'Access tokens',
        })}
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={openCreateDialog}
            aria-label={intl.formatMessage({
              id: 'pages.accessToken.settings.addToken',
              defaultMessage: 'Add access token',
            })}
          >
            {intl.formatMessage({
              id: 'pages.accessToken.settings.addToken',
              defaultMessage: 'Add access token',
            })}
          </Button>
        }
      >
        {error && (
          <Alert
            showIcon
            type="error"
            message={intl.formatMessage({
              id: 'pages.accessToken.settings.loadFailed',
              defaultMessage: 'Unable to load access tokens.',
            })}
            action={
              <Button
                size="small"
                icon={<ReloadOutlined />}
                onClick={() => void refreshTokens().catch(() => undefined)}
                aria-label={intl.formatMessage({
                  id: 'pages.accessToken.settings.retry',
                  defaultMessage: 'Retry',
                })}
              >
                {intl.formatMessage({
                  id: 'pages.accessToken.settings.retry',
                  defaultMessage: 'Retry',
                })}
              </Button>
            }
            style={{ marginBottom: 16 }}
          />
        )}
        {operationNotice && (
          <Alert
            showIcon
            closable
            type={operationNotice.type}
            message={operationNotice.message}
            onClose={() => setOperationNotice(undefined)}
            style={{ marginBottom: 16 }}
          />
        )}
        {renderTokenList()}
      </Card>

      <Modal
        open={createOpen}
        title={intl.formatMessage({
          id: 'pages.accessToken.settings.addToken',
          defaultMessage: 'Add access token',
        })}
        okText={intl.formatMessage({
          id: 'pages.accessToken.settings.createAction',
          defaultMessage: 'Create token',
        })}
        cancelText={intl.formatMessage({
          id: 'pages.title.cancel',
          defaultMessage: 'Cancel',
        })}
        confirmLoading={creating}
        closable={!creating}
        maskClosable={!creating}
        cancelButtonProps={{ disabled: creating }}
        onOk={() => createForm.submit()}
        onCancel={closeCreateDialog}
        destroyOnClose
      >
        {createError && (
          <Alert showIcon type="error" message={createError} style={{ marginBottom: 16 }} />
        )}
        <Form<API.postUserAuthTokenGenerateParams>
          form={createForm}
          layout="vertical"
          onFinish={(values) => void createToken(values)}
          preserve={false}
        >
          <Form.Item
            name="validityPeriod"
            initialValue="24h"
            label={intl.formatMessage({
              id: 'pages.accessToken.settings.validityPeriod',
              defaultMessage: 'Validity period',
            })}
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.accessToken.settings.validityRequired',
                  defaultMessage: 'Select a validity period.',
                }),
              },
            ]}
          >
            <Select
              options={[
                {
                  value: '24h',
                  label: intl.formatMessage({
                    id: 'pages.accessToken.settings.oneDay',
                    defaultMessage: 'One day',
                  }),
                },
                {
                  value: '168h',
                  label: intl.formatMessage({
                    id: 'pages.accessToken.settings.sevenDay',
                    defaultMessage: 'Seven days',
                  }),
                },
                {
                  value: '720h',
                  label: intl.formatMessage({
                    id: 'pages.accessToken.settings.thirtyDay',
                    defaultMessage: 'Thirty days',
                  }),
                },
                {
                  value: '2160h',
                  label: intl.formatMessage({
                    id: 'pages.accessToken.settings.ninetyDay',
                    defaultMessage: 'Ninety days',
                  }),
                },
                {
                  value: '0',
                  label: intl.formatMessage({
                    id: 'pages.accessToken.settings.noExpired',
                    defaultMessage: 'Never expires',
                  }),
                },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={Boolean(oneTimeSecret)}
        title={intl.formatMessage({
          id: 'pages.accessToken.settings.oneTimeTitle',
          defaultMessage: 'Copy your new access token',
        })}
        onCancel={closeOneTimeSecret}
        closable={!copying}
        maskClosable={false}
        keyboard={false}
        destroyOnClose
        footer={
          <Button type="primary" disabled={copying} onClick={closeOneTimeSecret}>
            {intl.formatMessage({
              id: 'pages.accessToken.settings.oneTimeClose',
              defaultMessage: 'I have saved the token',
            })}
          </Button>
        }
      >
        <Alert
          showIcon
          type="warning"
          message={intl.formatMessage({
            id: 'pages.accessToken.settings.oneTimeWarning',
            defaultMessage: 'This token is shown only once.',
          })}
          description={intl.formatMessage({
            id: 'pages.accessToken.settings.oneTimeDescription',
            defaultMessage: 'Copy it now. You cannot view it again after closing this dialog.',
          })}
          style={{ marginBottom: 16 }}
        />
        <Typography.Paragraph>
          <Input.TextArea
            readOnly
            autoSize={{ minRows: 3, maxRows: 6 }}
            value={oneTimeSecret || ''}
            aria-label={intl.formatMessage({
              id: 'pages.accessToken.settings.oneTimeInputLabel',
              defaultMessage: 'One-time personal access token',
            })}
          />
        </Typography.Paragraph>
        <Space>
          <Button
            icon={<CopyOutlined />}
            loading={copying}
            onClick={() => void copyOneTimeSecret()}
            aria-label={intl.formatMessage({
              id: 'pages.accessToken.settings.copy',
              defaultMessage: 'Copy token',
            })}
          >
            {intl.formatMessage({
              id: 'pages.accessToken.settings.copy',
              defaultMessage: 'Copy token',
            })}
          </Button>
          {copyStatus && (
            <Typography.Text role="status" type={copyStatus === 'success' ? 'success' : 'danger'}>
              {intl.formatMessage({
                id:
                  copyStatus === 'success'
                    ? 'pages.accessToken.settings.copySuccess'
                    : 'pages.accessToken.settings.copyFailed',
                defaultMessage: copyStatus === 'success' ? 'Copied' : 'Copy failed',
              })}
            </Typography.Text>
          )}
        </Space>
      </Modal>
    </>
  );
};

export default AccessTokenView;
