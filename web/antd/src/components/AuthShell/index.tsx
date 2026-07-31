import { useEmotionCss } from '@ant-design/use-emotion-css';
import { Helmet, SelectLang } from '@umijs/max';
import React, { PropsWithChildren } from 'react';
import Settings from '../../../config/defaultSettings';

interface AuthShellProps {
  titleDefaultMessage: string;
  showLang?: boolean;
}

const AuthShell: React.FC<PropsWithChildren<AuthShellProps>> = ({
  children,
  titleDefaultMessage,
  showLang = true,
}) => {
  const containerClassName = useEmotionCss(() => ({
    display: 'flex',
    flexDirection: 'column',
    height: '100vh',
    overflow: 'auto',
    backgroundImage:
      "url('https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/V-_oS6r-i7wAAAAAAAAAAAAAFl94AQBr')",
    backgroundSize: '100% 100%',
  }));

  const langClassName = useEmotionCss(({ token }) => ({
    width: 42,
    height: 42,
    lineHeight: '42px',
    position: 'fixed',
    right: 16,
    borderRadius: token.borderRadius,
    ':hover': {
      backgroundColor: token.colorBgTextHover,
    },
    'path.fill': '#555',
  }));

  const Lang = () => (
    <div className={langClassName} data-lang>
      {SelectLang && <SelectLang />}
    </div>
  );

  return (
    <div className={containerClassName}>
      <Helmet>
        <title>
          {titleDefaultMessage} - {Settings.title}
        </title>
      </Helmet>
      {showLang && <Lang />}
      <div
        style={{
          flex: '1',
          padding: '32px 0',
        }}
      >
        {children}
      </div>
    </div>
  );
};

export default AuthShell;
