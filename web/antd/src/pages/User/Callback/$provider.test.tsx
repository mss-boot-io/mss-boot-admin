import { render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import { postUserProviderCallback } from '@/services/admin/user';
import { publishOAuthCallbackResult } from '@/utils/oauth';
import CallbackPage, { consumeOAuthCallbackParams } from './$provider';

jest.mock('@/services/admin/user', () => ({
  postUserProviderCallback: jest.fn(),
}));

jest.mock('@/utils/oauth', () => ({
  publishOAuthCallbackResult: jest.fn(),
  requireServerOAuthIntent: jest.fn((response) => response.intent),
}));

jest.mock('@umijs/max', () => ({
  useIntl: () => ({ formatMessage: ({ id }: { id: string }) => id }),
  useParams: () => ({ provider: 'github' }),
}));

jest.mock('antd', () => {
  const ReactRuntime = require('react');
  return {
    Alert: (props: any) => ReactRuntime.createElement('div', props),
    Flex: ({ children }: any) => ReactRuntime.createElement('div', null, children),
    Spin: (props: any) =>
      ReactRuntime.createElement('div', {
        'aria-label': props['aria-label'],
        'data-fullscreen': props.fullscreen ? 'true' : 'false',
        'data-testid': 'callback-spinner',
        'data-tip': props.tip,
      }),
    message: { error: jest.fn(), success: jest.fn() },
  };
});

const mockedCallback = postUserProviderCallback as jest.Mock;
const mockedPublish = publishOAuthCallbackResult as jest.Mock;

describe('OAuth callback page', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    localStorage.clear();
    window.history.pushState(
      {},
      '',
      '/user/callback/github?code=provider-code&state=server-state&keep=safe#done',
    );
  });

  it('keeps the first consumed values when React initializes twice in development', () => {
    const first = consumeOAuthCallbackParams();
    const second = consumeOAuthCallbackParams();

    expect(first).toEqual({ code: 'provider-code', state: 'server-state' });
    expect(second).toEqual(first);
    expect(window.location.search).toBe('?keep=safe');
  });

  it('scrubs code and state immediately, posts JSON, and broadcasts only the safe response', async () => {
    const response: API.OAuthCallbackResponse = {
      attemptID: 'attempt-callback',
      code: 200,
      provider: 'github',
      intent: 'login',
      token: 'admin-session-token',
      expire: '2026-08-06T12:05:00Z',
    };
    mockedCallback.mockResolvedValue(response);

    render(
      <React.StrictMode>
        <CallbackPage />
      </React.StrictMode>,
    );

    const spinner = screen.getByTestId('callback-spinner');
    expect(spinner.getAttribute('aria-label')).toBe('pages.login.callback.loading');
    expect(spinner.getAttribute('data-fullscreen')).toBe('true');
    expect(spinner.hasAttribute('data-tip')).toBe(false);

    expect(window.location.pathname).toBe('/user/callback/github');
    expect(window.location.search).toBe('?keep=safe');
    expect(window.location.hash).toBe('#done');
    expect(window.location.href).not.toContain('provider-code');
    expect(window.location.href).not.toContain('server-state');

    await waitFor(() => {
      expect(mockedCallback).toHaveBeenCalledWith(
        { provider: 'github' },
        { code: 'provider-code', state: 'server-state' },
        { skipErrorHandler: true },
      );
      expect(mockedPublish).toHaveBeenCalledWith(response);
    });
    expect(mockedCallback).toHaveBeenCalledTimes(1);
    expect(mockedPublish).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId('callback-spinner')).toBeNull();
    expect(localStorage.setItem).not.toHaveBeenCalled();
    expect(JSON.stringify(mockedPublish.mock.calls)).not.toContain('accessToken');
    expect(JSON.stringify(mockedPublish.mock.calls)).not.toContain('refreshToken');
  });
});
