import React, { useEffect, useRef, useState } from 'react';
import { Avatar, Spin, Tag, message, Row, Col, Space, Typography } from 'antd';
import { UserOutlined, TeamOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { useIntl, useModel } from '@umijs/max';
import { PageContainer, ProCard, ProDescriptions } from '@ant-design/pro-components';
import { province } from '../Settings/geographic/province';
import { city } from '../Settings/geographic/city';
import { useResponsive } from '@/hooks/useResponsive';
import MobileCenter from './Mobile';
import { getAccountCenterDetails, getAccountCenterUser } from './details';

const { Title, Paragraph } = Typography;

const Center: React.FC = () => {
  const intl = useIntl();
  const { isMobile } = useResponsive();
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const [userInfo, setUserInfo] = useState<API.User | undefined>(currentUser);
  const [departmentInfo, setDepartmentInfo] = useState<API.Department>();
  const [postInfo, setPostInfo] = useState<API.Post>();
  const [loading, setLoading] = useState(false);
  const [messageApi, contextHolder] = message.useMessage({
    top: 60,
    duration: 3,
    maxCount: 3,
  });
  const messageApiRef = useRef(messageApi);
  const intlRef = useRef(intl);

  useEffect(() => {
    messageApiRef.current = messageApi;
    intlRef.current = intl;
  }, [intl, messageApi]);

  useEffect(() => {
    let isMounted = true;
    const controller = new AbortController();

    const fetchData = async () => {
      setLoading(true);
      setDepartmentInfo(undefined);
      setPostInfo(undefined);

      try {
        // The initial state is populated at sign-in and is shared by the header,
        // access checks, and this page. Only fall back to the API if it is absent.
        const user = await getAccountCenterUser(currentUser, controller.signal);
        if (!isMounted) {
          return;
        }

        setUserInfo(user);
        const { departmentInfo: department, postInfo: post } = await getAccountCenterDetails(
          user,
          controller.signal,
        );
        if (!isMounted) {
          return;
        }

        setDepartmentInfo(department);
        setPostInfo(post);
      } catch (error) {
        if (isMounted && !controller.signal.aborted) {
          messageApiRef.current.error(
            intlRef.current.formatMessage({ id: 'pages.account.center.fetchDataError' }),
          );
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    void fetchData();

    return () => {
      isMounted = false;
      controller.abort();
    };
  }, [currentUser]);

  if (isMobile) {
    return (
      <PageContainer title={intl.formatMessage({ id: 'pages.account.center.title' })}>
        {contextHolder}
        <Spin spinning={loading}>
          <MobileCenter 
            userInfo={userInfo} 
            departmentInfo={departmentInfo} 
            postInfo={postInfo} 
          />
        </Spin>
      </PageContainer>
    );
  }

  return (
    <PageContainer title={intl.formatMessage({ id: 'pages.account.center.title' })}>
      {contextHolder}
      <Spin spinning={loading}>
        <Row gutter={24}>
          <Col span={8}>
            <ProCard>
              <Space
                direction="vertical"
                align="center"
                style={{ width: '100%', padding: '24px 0' }}
              >
                <Avatar size={100} src={userInfo?.avatar} icon={<UserOutlined />} />
                <Title level={2} style={{ marginTop: 16, marginBottom: 4 }}>
                  {userInfo?.name}
                </Title>
                <Paragraph type="secondary">{userInfo?.title}</Paragraph>
              </Space>
              <ProCard split="horizontal">
                <ProCard>
                  <Space align="start">
                    <TeamOutlined style={{ fontSize: 24 }} />
                    <Space direction="vertical" size={4}>
                      <span>{intl.formatMessage({ id: 'pages.account.center.department' })}</span>
                      <span>{departmentInfo?.name || '-'}</span>
                    </Space>
                  </Space>
                </ProCard>
                <ProCard>
                  <Space align="start">
                    <SafetyCertificateOutlined style={{ fontSize: 24 }} />
                    <Space direction="vertical" size={4}>
                      <span>{intl.formatMessage({ id: 'pages.account.center.post' })}</span>
                      <span>{postInfo?.name || '-'}</span>
                    </Space>
                  </Space>
                </ProCard>
              </ProCard>
            </ProCard>
          </Col>
          <Col span={16}>
            <ProCard>
              <ProDescriptions
                column={2}
                title={intl.formatMessage({ id: 'pages.account.center.title' })}
                dataSource={userInfo}
                columns={[
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.username' }),
                    dataIndex: 'username',
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.phone' }),
                    dataIndex: 'phone',
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.email' }),
                    dataIndex: 'email',
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.role' }),
                    dataIndex: ['role', 'name'],
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.address' }),
                    dataIndex: 'address',
                    span: 2,
                    render: () => {
                      const country = userInfo?.country
                        ? intl.formatMessage({
                            id: `pages.account.center.country.${userInfo?.country}`,
                          })
                        : '';
                      const provinceName = province.find((item: { id: string; name: string }) => {
                        return item.id === userInfo?.province;
                      })?.name;
                      const cityName =
                        userInfo?.province && userInfo?.city
                          ? city[userInfo.province as keyof typeof city].find(
                              (item: { id: string; name: string }) => {
                                return item.id === userInfo.city;
                              },
                            )?.name
                          : undefined;
                      return [country, provinceName, cityName, userInfo?.address]
                        .filter(Boolean)
                        .join(' ');
                    },
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.profile' }),
                    dataIndex: 'profile',
                    span: 2,
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.signature' }),
                    dataIndex: 'signature',
                    span: 2,
                  },
                  {
                    title: intl.formatMessage({ id: 'pages.account.center.tags' }),
                    dataIndex: 'tags',
                    span: 2,
                    render: (dom: any, entity: API.User) =>
                      entity.tags?.map((tag) => <Tag key={tag}>{tag}</Tag>),
                  },
                ]}
              />
            </ProCard>
          </Col>
        </Row>
      </Spin>
    </PageContainer>
  );
};

export default Center;
