import { QueryClientProvider } from '@tanstack/react-query';
import type { RequestConfig } from '@umijs/max';
import { useIntl } from '@umijs/max';
import { App as AntdApp } from 'antd';
import type { ReactNode } from 'react';
import { requestConfig } from './shared/api/client';
import { RuntimeFeedbackBridge } from './shared/api/feedback';
import AuthorizationFreshnessBridge from './shared/auth/AuthorizationFreshnessBridge';
import SessionRefreshBridge from './shared/auth/SessionRefreshBridge';
import { loadInitialState } from './shared/bootstrap/initialState';
import { RuntimeMessageProvider } from './shared/i18n/runtime';
import { runtimeLayout } from './shared/layout/RuntimeLayout';
import { queryClient } from './shared/query/client';
import AuthorizationRealtimeBridge from './shared/realtime/AuthorizationRealtimeBridge';
import { ThemeCrossTabBridge } from './shared/theme/ThemeCrossTabBridge';
import { ThemeRuntimeProvider } from './shared/theme/ThemeRuntimeProvider';

export const getInitialState = loadInitialState;

export const layout = runtimeLayout;

export const request: RequestConfig = requestConfig;

function RuntimeProviders({ children }: { children: ReactNode }) {
  const intl = useIntl();
  return (
    <RuntimeMessageProvider formatMessage={(messageID) => intl.formatMessage({ id: messageID })}>
      <ThemeRuntimeProvider>
        <AntdApp>
          <RuntimeFeedbackBridge />
          <QueryClientProvider client={queryClient}>
            <SessionRefreshBridge />
            <AuthorizationFreshnessBridge />
            <AuthorizationRealtimeBridge />
            <ThemeCrossTabBridge />
            {children}
          </QueryClientProvider>
        </AntdApp>
      </ThemeRuntimeProvider>
    </RuntimeMessageProvider>
  );
}

// Umi applies innerProvider before wrapping the application in its dataflow
// provider. The resulting RuntimeProviders subtree is therefore inside the
// @@initialState model context when it renders. Do not move model-consuming
// bridges to rootContainer, which is intentionally the outermost runtime hook.
export function innerProvider(container: ReactNode) {
  return <RuntimeProviders>{container}</RuntimeProviders>;
}
