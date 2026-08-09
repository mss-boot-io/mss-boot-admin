import { act, render, screen, waitFor } from '@testing-library/react';
import * as React from 'react';
import { getAppConfigsProfile } from '@/services/admin/appConfig';
import {
  EmailChallengeRoute,
  isEmailChallengeFlowAvailable,
} from './emailChallengeAccess';

let mockInitialState: any;
const mockSetInitialState = jest.fn((updater) => {
  mockInitialState = typeof updater === 'function' ? updater(mockInitialState) : updater;
});

jest.mock('@umijs/max', () => {
  const ReactRuntime = require('react');
  return {
    FormattedMessage: ({ defaultMessage, id }: { defaultMessage?: string; id: string }) =>
      ReactRuntime.createElement(ReactRuntime.Fragment, null, defaultMessage || id),
    Link: ({ children, to }: { children?: any; to: string }) =>
      ReactRuntime.createElement('a', { href: to }, children),
    useModel: () => ({ initialState: mockInitialState, setInitialState: mockSetInitialState }),
  };
});

jest.mock('@/components/AuthShell', () => ({ children }: { children?: any }) => (
  <main>{children}</main>
));
jest.mock('@/components/Footer', () => () => null);
jest.mock('@/services/admin/appConfig', () => ({
  getAppConfigsProfile: jest.fn(),
}));

describe('email Challenge access', () => {
  beforeEach(() => {
    mockInitialState = {
      appConfig: {
        security: {
          emailEnabled: true,
          emailChallengeReady: true,
          registerEnabled: true,
        },
      },
    };
    jest.clearAllMocks();
  });

  afterEach(() => {
    jest.useRealTimers();
  });

  it('requires readiness and the flow-specific feature flags', () => {
    const profile = {
      security: {
        emailEnabled: true,
        emailChallengeReady: true,
        registerEnabled: false,
      },
    } as API.AppConfigProfile;

    expect(isEmailChallengeFlowAvailable(profile, 'login')).toBe(true);
    expect(isEmailChallengeFlowAvailable(profile, 'resetPassword')).toBe(true);
    expect(isEmailChallengeFlowAvailable(profile, 'register')).toBe(false);
    expect(
      isEmailChallengeFlowAvailable(
        { security: { ...profile.security, emailChallengeReady: false } },
        'login',
      ),
    ).toBe(false);
    expect(isEmailChallengeFlowAvailable(undefined, 'login')).toBe(false);
  });

  it('does not render a protected route while the live check is pending', () => {
    let resolveProfile: ((profile: API.AppConfigProfile) => void) | undefined;
    (getAppConfigsProfile as jest.Mock).mockReturnValue(
      new Promise<API.AppConfigProfile>((resolve) => {
        resolveProfile = resolve;
      }),
    );

    const view = render(
      <EmailChallengeRoute flow="resetPassword">
        <div>protected recovery form</div>
      </EmailChallengeRoute>,
    );

    expect(screen.getByRole('status')).toBeTruthy();
    expect(screen.queryByText('protected recovery form')).toBeNull();
    view.unmount();
    resolveProfile?.({ security: {} });
  });

  it('fails closed and clears stale readiness when the refresh fails', async () => {
    (getAppConfigsProfile as jest.Mock).mockRejectedValue(new Error('profile unavailable'));

    render(
      <EmailChallengeRoute flow="resetPassword">
        <div>protected recovery form</div>
      </EmailChallengeRoute>,
    );

    await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
    expect(screen.queryByText('protected recovery form')).toBeNull();
    expect(mockInitialState.appConfig.security.emailChallengeReady).toBe(false);
    expect(screen.getByRole('link', { name: '返回登录' }).getAttribute('href')).toBe(
      '/user/login',
    );
  });

  it('leaves the checking state after a bounded profile timeout', async () => {
    jest.useFakeTimers();
    (getAppConfigsProfile as jest.Mock).mockReturnValue(
      new Promise(() => {
        // Intentionally unresolved: the hook's own deadline must settle the UI.
      }),
    );

    render(
      <EmailChallengeRoute flow="resetPassword">
        <div>protected recovery form</div>
      </EmailChallengeRoute>,
    );

    expect(screen.getByRole('status')).toBeTruthy();
    await act(async () => {
      jest.advanceTimersByTime(4000);
      await Promise.resolve();
    });

    expect(screen.getByRole('alert')).toBeTruthy();
    expect(screen.queryByText('protected recovery form')).toBeNull();
    expect(getAppConfigsProfile).toHaveBeenCalledWith({
      skipErrorHandler: true,
      timeout: 4000,
    });
  });

  it('keeps the registration route closed when registration is disabled', async () => {
    (getAppConfigsProfile as jest.Mock).mockResolvedValue({
      security: {
        emailEnabled: true,
        emailChallengeReady: true,
        registerEnabled: false,
      },
    });

    render(
      <EmailChallengeRoute flow="register">
        <div>protected registration form</div>
      </EmailChallengeRoute>,
    );

    await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
    expect(screen.getByText('当前未开放用户注册。')).toBeTruthy();
    expect(screen.queryByText('protected registration form')).toBeNull();
  });

  it('renders a direct route only after a fresh ready response', async () => {
    (getAppConfigsProfile as jest.Mock).mockResolvedValue({
      security: {
        emailEnabled: true,
        emailChallengeReady: true,
        registerEnabled: true,
      },
    });

    render(
      <EmailChallengeRoute flow="resetPassword">
        <div>protected recovery form</div>
      </EmailChallengeRoute>,
    );

    await waitFor(() => expect(screen.getByText('protected recovery form')).toBeTruthy());
    expect(screen.queryByRole('alert')).toBeNull();
  });
});
