import { GithubOutlined } from '@ant-design/icons';
import { DefaultFooter } from '@ant-design/pro-components';
import { useIntl, useModel } from '@umijs/max';
import React from 'react';

const Footer: React.FC = () => {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState');

  const defaultMessage =
    initialState?.appConfig?.base?.websiteCopyRight ||
    intl.formatMessage({ id: 'app.copyright.produced' });

  const currentYear = new Date().getFullYear();

  return (
    <DefaultFooter
      style={{
        background: 'none',
      }}
      copyright={`${currentYear} ${defaultMessage}`}
      links={[
        {
          key: 'beian',
          title: initialState?.appConfig?.base?.websiteRecordNumber,
          href: 'http://beian.miit.gov.cn',
          blankTarget: true,
        },
        {
          key: 'github',
          title: <GithubOutlined />,
          href: 'https://github.com/mss-boot-io/mss-boot',
          blankTarget: true,
        },
        {
          key: 'mss-boot-admin',
          title: 'mss-boot-admin',
          href: 'https://github.com/mss-boot-io/mss-boot-admin',
          blankTarget: true,
        },
      ]}
    />
  );
};

export default Footer;
