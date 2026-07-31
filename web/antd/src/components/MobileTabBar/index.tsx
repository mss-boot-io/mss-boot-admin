import { HomeOutlined, SettingOutlined, UserOutlined } from '@ant-design/icons';
import { history, useLocation } from '@umijs/max';
import React from 'react';
import styles from './index.less';

interface TabItem {
  key: string;
  title: string;
  icon: React.ReactNode;
}

const tabs: TabItem[] = [
  {
    key: '/',
    title: '首页',
    icon: <HomeOutlined />,
  },
  {
    key: '/user',
    title: '用户',
    icon: <UserOutlined />,
  },
  {
    key: '/account/center',
    title: '我的',
    icon: <UserOutlined />,
  },
  {
    key: '/account/settings',
    title: '设置',
    icon: <SettingOutlined />,
  },
];

const MobileTabBar: React.FC = () => {
  const location = useLocation();
  const { pathname } = location;

  return (
    <div className={styles.mobileTabBar}>
      <div className={styles.tabBarContent}>
        {tabs.map((item) => {
          const isActive = pathname === item.key;
          return (
            <div
              key={item.key}
              className={`${styles.tabItem} ${isActive ? styles.active : ''}`}
              onClick={() => history.push(item.key)}
            >
              <div className={styles.icon}>{item.icon}</div>
              <div className={styles.title}>{item.title}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default MobileTabBar;
