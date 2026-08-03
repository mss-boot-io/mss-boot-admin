import React, { useEffect, useState } from 'react';
import { Avatar, Spin, Tag, message, Row, Col, Space, Typography } from 'antd';
import { UserOutlined, TeamOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import { getUserUserInfo } from '@/services/admin/user';
import { getDepartmentsId } from '@/services/admin/department';
import { getPostsId } from '@/services/admin/post';
import { useIntl } from '@umijs/max';
import { PageContainer, ProCard, ProDescriptions } from '@ant-design/pro-components';
import { province } from '../Settings/geographic/province';
import { city } from '../Settings/geographic/city';
import { useResponsive } from '@/hooks/useResponsive';
import MobileCenter from './Mobile';

const { Title, Paragraph } = Typography;

const Center: React.FC = () => {
  const intl = useIntl();
  const { isMobile } = useResponsive();
  const [userInfo, setUserInfo] = useState<API.User>();
  const [departmentInfo, setDepartmentInfo] = useState<API.Department>();
  const [postInfo, setPostInfo] = useState<API.Post>();
  const [loading, setLoading] = useState(false);
  const [messageApi, contextHolder] = message.useMessage({
    top: 60,
    duration: 3,
    maxCount: 3,
  });

  const fetchData = async () => {
    try {
      setLoading(true);
      const userRes = await getUserUserInfo();
      setUserInfo(userRes);

      if (userRes.department?.id) {
        const deptRes = await getDepartmentsId({ id: userRes.department.id });
        setDepartmentInfo(deptRes);
      }

      if (userRes.post?.id) {
        const postRes = await getPostsId({ id: userRes.post.id });
        setPostInfo(postRes);
      }
    } catch (error) {
      messageApi.error(intl.formatMessage({ id: 'pages.account.center.fetchDataError' }));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

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
