import ArrowLeftOutlined from '@ant-design/icons/ArrowLeftOutlined';
import { Link, SelectLang, useIntl, useModel } from '@umijs/max';
import { Button, Card, Image, Space, Typography } from 'antd';
import type { ReactNode } from 'react';
import type { InitialState } from '@/shared/auth/types';

export interface PublicAuthFrameProps {
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  showBackToLogin?: boolean;
}

export default function PublicAuthFrame({
  title,
  description,
  children,
  showBackToLogin = true,
}: PublicAuthFrameProps) {
  const intl = useIntl();
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const brandTitle =
    typeof initialState?.settings.title === 'string' && initialState.settings.title.trim()
      ? initialState.settings.title
      : 'mss-boot-io';
  const brandLogo =
    typeof initialState?.settings.logo === 'string' && initialState.settings.logo.trim()
      ? initialState.settings.logo
      : '/logo.svg';

  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top_left,var(--mss-color-primary-bg),transparent_42%),var(--mss-color-bg-layout)] px-4 py-10">
      <div className="fixed right-4 top-4 z-10 rounded-lg bg-[var(--mss-color-bg-container)] shadow-sm">
        <SelectLang />
      </div>
      <div className="mx-auto w-full max-w-xl pt-8 sm:pt-14">
        <Space
          className="mb-6 w-full justify-center"
          orientation="vertical"
          align="center"
          size={8}
        >
          <Image preview={false} src={brandLogo} alt={brandTitle} width={56} height={56} />
          <Typography.Title level={2} className="m-0 text-center">
            {brandTitle}
          </Typography.Title>
        </Space>
        <Card
          styles={{ body: { padding: 'clamp(20px, 5vw, 40px)' } }}
          className="border-[var(--mss-color-border-secondary)] shadow-sm"
        >
          <Typography.Title level={3} className="mt-0 text-center">
            {title}
          </Typography.Title>
          {description ? (
            <Typography.Paragraph className="mb-8 text-center" type="secondary">
              {description}
            </Typography.Paragraph>
          ) : null}
          {children}
          {showBackToLogin ? (
            <div className="mt-6 text-center">
              <Link to="/user/login">
                <Button type="link" icon={<ArrowLeftOutlined />}>
                  {intl.formatMessage({ id: 'pages.auth.backToLogin' })}
                </Button>
              </Link>
            </div>
          ) : null}
        </Card>
      </div>
    </main>
  );
}
