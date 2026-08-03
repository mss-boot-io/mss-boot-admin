import React from 'react';
import { Avatar, Card, Space, Typography, Tag } from 'antd';
import { UserOutlined, TeamOutlined, SafetyCertificateOutlined, MailOutlined, PhoneOutlined, EnvironmentOutlined } from '@ant-design/icons';
import { useIntl } from '@umijs/max';
import { province } from '../../Settings/geographic/province';
import { city } from '../../Settings/geographic/city';
import styles from '@/styles/mobile.less';

const { Title, Text } = Typography;

interface MobileCenterProps {
  userInfo?: API.User;
  departmentInfo?: API.Department;
  postInfo?: API.Post;
}

const MobileCenter: React.FC<MobileCenterProps> = ({ userInfo, departmentInfo, postInfo }) => {
  const intl = useIntl();

  const getAddress = () => {
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
  };

  return (
    <div className={styles.mobileContainer}>
      <Card className={styles.card}>
        <Space direction="vertical" align="center" style={{ width: '100%' }}>
          <Avatar size={80} src={userInfo?.avatar} icon={<UserOutlined />} />
          <Title level={3} style={{ margin: '12px 0 4px' }}>
            {userInfo?.name || '-'}
          </Title>
          <Text type="secondary">{userInfo?.title || '-'}</Text>
        </Space>
      </Card>

      <Card className={styles.card} title={intl.formatMessage({ id: 'pages.account.center.department' })}>
        <div className={styles.cardBody}>
          <div className={styles.field}>
            <span className={styles.label}>
              <TeamOutlined style={{ marginRight: 8 }} />
              {intl.formatMessage({ id: 'pages.account.center.department' })}
            </span>
            <span className={styles.value}>{departmentInfo?.name || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              <SafetyCertificateOutlined style={{ marginRight: 8 }} />
              {intl.formatMessage({ id: 'pages.account.center.post' })}
            </span>
            <span className={styles.value}>{postInfo?.name || '-'}</span>
          </div>
        </div>
      </Card>

      <Card className={styles.card} title={intl.formatMessage({ id: 'pages.account.center.title' })}>
        <div className={styles.cardBody}>
          <div className={styles.field}>
            <span className={styles.label}>
              {intl.formatMessage({ id: 'pages.account.center.username' })}
            </span>
            <span className={styles.value}>{userInfo?.username || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              <PhoneOutlined style={{ marginRight: 8 }} />
              {intl.formatMessage({ id: 'pages.account.center.phone' })}
            </span>
            <span className={styles.value}>{userInfo?.phone || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              <MailOutlined style={{ marginRight: 8 }} />
              {intl.formatMessage({ id: 'pages.account.center.email' })}
            </span>
            <span className={styles.value}>{userInfo?.email || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              {intl.formatMessage({ id: 'pages.account.center.role' })}
            </span>
            <span className={styles.value}>{userInfo?.role?.name || '-'}</span>
          </div>
        </div>
      </Card>

      <Card className={styles.card} title={intl.formatMessage({ id: 'pages.account.center.address' })}>
        <div className={styles.cardBody}>
          <div className={styles.field}>
            <span className={styles.label}>
              <EnvironmentOutlined style={{ marginRight: 8 }} />
              {intl.formatMessage({ id: 'pages.account.center.address' })}
            </span>
            <span className={styles.value}>{getAddress() || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              {intl.formatMessage({ id: 'pages.account.center.profile' })}
            </span>
            <span className={styles.value}>{userInfo?.profile || '-'}</span>
          </div>
          <div className={styles.field}>
            <span className={styles.label}>
              {intl.formatMessage({ id: 'pages.account.center.signature' })}
            </span>
            <span className={styles.value}>{userInfo?.signature || '-'}</span>
          </div>
          {userInfo?.tags && userInfo.tags.length > 0 && (
            <div className={styles.field}>
              <span className={styles.label}>
                {intl.formatMessage({ id: 'pages.account.center.tags' })}
              </span>
              <Space wrap style={{ flex: 1, justifyContent: 'flex-end' }}>
                {userInfo.tags.map((tag) => (
                  <Tag key={tag}>{tag}</Tag>
                ))}
              </Space>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};

export default MobileCenter;