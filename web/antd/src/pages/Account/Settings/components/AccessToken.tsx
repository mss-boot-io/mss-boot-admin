import React, { PropsWithChildren, useState } from 'react';
import { message, Card, Empty } from 'antd';
import { useRequest } from 'ahooks';
import { useIntl } from '@umijs/max';
import {
  getUserAuthTokenGenerate,
  getUserAuthTokens,
  putUserAuthTokenIdRevoke,
} from '@/services/admin/userAuthToken';
import { CopyOutlined, PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { ProForm, ModalForm, ProFormSelect } from '@ant-design/pro-components';

// 自定义日期解析和格式化函数
const formatDateTime = (dateString: string) => {
  const date = new Date(dateString);

  // 获取年、月、日、时、分、秒
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

const CopyButton: React.FC<PropsWithChildren<{ textToCopy: string }>> = (props) => {
  const [copySuccess, setCopySuccess] = useState('');
  const intl = useIntl();

  const copyToClipboard = () => {
    navigator.clipboard
      .writeText(props.textToCopy)
      .then(() => {
        setCopySuccess(
          intl.formatMessage({
            id: 'pages.accessToken.settings.copySuccess',
            defaultMessage: '已复制',
          }),
        );
      })
      .catch((err) => {
        setCopySuccess(
          intl.formatMessage({
            id: 'pages.accessToken.settings.copyFailed',
            defaultMessage: '复制失败',
          }),
        );
        console.error('Failed to copy: ', err);
      });
  };

  return (
    <div>
      {intl.formatMessage({
        id: 'pages.accessToken.settings.token',
        defaultMessage: '令牌',
      })}
      : **********
      <CopyOutlined onClick={copyToClipboard} />
      {copySuccess && <span>{copySuccess}</span>}
    </div>
  );
};

const AccessTokenView: React.FC = () => {
  /**
   * @en-US International configuration
   * @zh-CN 国际化配置
   * */
  const intl = useIntl();

  const [addToken, setAddToken] = useState<boolean>(false);
  const [refresh, setRefresh] = useState(0);

  const now = new Date();
  const oneYearLater = new Date(now).setFullYear(now.getFullYear() + 1);

  const { data: accessTokenDatas, loading } = useRequest(
    async () => {
      const res = await getUserAuthTokens();
      return res.data || [];
    },
    {
      refreshDeps: [refresh],
    },
  );

  return (
    <>
      {loading ? (
        <></>
      ) : (
        <Card
          title={intl.formatMessage({
            id: 'pages.accessToken.settings.title',
            defaultMessage: '访问令牌',
          })}
          extra={<PlusOutlined onClick={() => setAddToken(true)} />}
        >
          {accessTokenDatas && accessTokenDatas.length > 0 ? (
            accessTokenDatas.map((item: any) => (
              <Card
                key={item.id}
                type="inner"
                title={`${intl.formatMessage({
                  id: 'pages.accessToken.settings.id',
                  defaultMessage: 'ID',
                })}: ${item.id}`}
                extra={
                  <DeleteOutlined
                    onClick={async () => {
                      await putUserAuthTokenIdRevoke({ id: item.id });
                      setRefresh(refresh + 1);
                    }}
                  />
                }
              >
                <CopyButton textToCopy={item.token} />
                <div>
                  {intl.formatMessage({
                    id: 'pages.accessToken.settings.expired',
                    defaultMessage: '过期时间',
                  })}
                  :{' '}
                  {new Date(item.expiredAt).getTime() > oneYearLater
                    ? intl.formatMessage({
                        id: 'pages.accessToken.settings.longTime',
                        defaultMessage: '长期有效',
                      })
                    : formatDateTime(item.expiredAt)}
                </div>
              </Card>
            ))
          ) : (
            <Empty
              description={intl.formatMessage({
                id: 'pages.accessToken.settings.noData',
                defaultMessage: '暂无访问令牌',
              })}
            />
          )}
        </Card>
      )}
      <ModalForm
        title={intl.formatMessage({
          id: 'pages.accessToken.settings.addToken',
          defaultMessage: '添加令牌',
        })}
        open={addToken}
        onFinish={async (item: API.getUserAuthTokenGenerateParams) => {
          await getUserAuthTokenGenerate(item);
          message.success(
            intl.formatMessage({
              id: 'pages.accessToken.settings.createSuccess',
              defaultMessage: '创建成功',
            }),
          );
          setRefresh(refresh + 1);
          return true;
        }}
        onOpenChange={setAddToken}
      >
        <ProForm.Group>
          <ProFormSelect
            name="validityPeriod"
            width="md"
            label={intl.formatMessage({
              id: 'pages.accessToken.settings.validityPeriod',
              defaultMessage: '有效期',
            })}
            valueEnum={{
              '24h': {
                text: intl.formatMessage({
                  id: 'pages.accessToken.settings.oneDay',
                  defaultMessage: '一天',
                }),
              },
              '168h': {
                text: intl.formatMessage({
                  id: 'pages.accessToken.settings.sevenDay',
                  defaultMessage: '七天',
                }),
              },
              '720h': {
                text: intl.formatMessage({
                  id: 'pages.accessToken.settings.thirtyDay',
                  defaultMessage: '三十天',
                }),
              },
              '2160h': {
                text: intl.formatMessage({
                  id: 'pages.accessToken.settings.ninetyDay',
                  defaultMessage: '九十天',
                }),
              },
              '0': {
                text: intl.formatMessage({
                  id: 'pages.accessToken.settings.noExpired',
                  defaultMessage: '永久有效',
                }),
              },
            }}
          />
        </ProForm.Group>
      </ModalForm>
    </>
  );
};

export default AccessTokenView;
