import * as React from 'react';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import AccessTokenView from './AccessToken';
import {
  getUserAuthTokens,
  postUserAuthTokenGenerate,
  putUserAuthTokenIdRefresh,
  putUserAuthTokenIdRevoke,
} from '@/services/admin/userAuthToken';

jest.mock('@/services/admin/userAuthToken', () => ({
  getUserAuthTokens: jest.fn(),
  postUserAuthTokenGenerate: jest.fn(),
  putUserAuthTokenIdRefresh: jest.fn(),
  putUserAuthTokenIdRevoke: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) =>
      defaultMessage || id,
  }),
}));

const mockedGetTokens = getUserAuthTokens as jest.Mock;
const mockedCreateToken = postUserAuthTokenGenerate as jest.Mock;
const mockedRotateToken = putUserAuthTokenIdRefresh as jest.Mock;
const mockedRevokeToken = putUserAuthTokenIdRevoke as jest.Mock;

const summary: API.UserAuthTokenSummary = {
  id: 'pat-1',
  userID: 'user-1',
  fingerprint: 'sha256:abcd1234',
  expiredAt: '2027-01-01T00:00:00Z',
  revoked: false,
};

const page = (data: API.UserAuthTokenSummary[]) => ({ data } as any);

const openCreateDialog = async () => {
  fireEvent.click(await screen.findByRole('button', { name: 'Add access token' }));
  return screen.findByRole('dialog', { name: 'Add access token' });
};

const submitCreateDialog = async () => {
  const dialog = await openCreateDialog();
  fireEvent.click(within(dialog).getByRole('button', { name: 'Create token' }));
};

describe('AccessTokenView', () => {
  let writeText: jest.Mock;

  beforeEach(() => {
    jest.clearAllMocks();
    mockedGetTokens.mockReset();
    mockedCreateToken.mockReset();
    mockedRotateToken.mockReset();
    mockedRevokeToken.mockReset();
    mockedGetTokens.mockResolvedValue(page([]));
    mockedCreateToken.mockResolvedValue({
      ...summary,
      token: 'pat-secret-once',
    });
    mockedRotateToken.mockResolvedValue({
      ...summary,
      fingerprint: 'sha256:rotated',
      token: 'pat-rotated-secret-once',
    });
    mockedRevokeToken.mockResolvedValue({} as any);
    writeText = jest.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
  });

  it('renders an explicit loading state and then an empty state', async () => {
    let resolveList: ((value: any) => void) | undefined;
    mockedGetTokens.mockReturnValue(
      new Promise((resolve) => {
        resolveList = resolve;
      }) as any,
    );

    render(<AccessTokenView />);

    expect(screen.getByLabelText('Loading access tokens')).toBeTruthy();

    await act(async () => {
      resolveList?.(page([]));
    });

    expect(await screen.findByText('No access tokens')).toBeTruthy();
  });

  it('renders summary metadata without exposing or copying a list token', async () => {
    mockedGetTokens.mockResolvedValue(
      page([{ ...summary, token: 'legacy-token-must-not-render' } as API.UserAuthTokenSummary]),
    );

    render(<AccessTokenView />);

    expect(await screen.findByText(/pat-1/)).toBeTruthy();
    expect(screen.getByText(/sha256:abcd1234/)).toBeTruthy();
    expect(screen.queryByText('legacy-token-must-not-render')).toBeNull();
    expect(screen.queryByDisplayValue('legacy-token-must-not-render')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Copy token' })).toBeNull();
  });

  it('shows a list error and retries the request', async () => {
    mockedGetTokens
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(page([summary]));

    render(<AccessTokenView />);

    expect(await screen.findByText('Unable to load access tokens.')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    expect(await screen.findByText(/pat-1/)).toBeTruthy();
    expect(mockedGetTokens).toHaveBeenCalledTimes(2);
    expect(mockedGetTokens).toHaveBeenLastCalledWith({ skipErrorHandler: true });
  });

  it('shows a created secret once and clears it after the dialog closes', async () => {
    render(<AccessTokenView />);

    await submitCreateDialog();

    await waitFor(() => {
      expect(mockedCreateToken).toHaveBeenCalledWith(
        { validityPeriod: '24h' },
        { skipErrorHandler: true },
      );
    });
    const secretInput = await screen.findByLabelText('One-time personal access token');
    expect((secretInput as HTMLTextAreaElement).value).toBe('pat-secret-once');
    expect(screen.getByText('This token is shown only once.')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: 'I have saved the token' }));

    await waitFor(() => {
      expect(screen.queryByDisplayValue('pat-secret-once')).toBeNull();
    });
    expect(screen.queryByRole('button', { name: 'Copy token' })).toBeNull();
  });

  it('reports copy success and failure without putting the token anywhere else', async () => {
    writeText.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error('denied'));
    render(<AccessTokenView />);
    await submitCreateDialog();

    const copyButton = await screen.findByRole('button', { name: 'Copy token' });
    fireEvent.click(copyButton);
    expect((await screen.findByRole('status')).textContent).toContain('Copied');
    expect(writeText).toHaveBeenLastCalledWith('pat-secret-once');

    fireEvent.click(copyButton);
    await waitFor(() => {
      expect(screen.getByRole('status').textContent).toContain('Copy failed');
    });
    expect(screen.getAllByDisplayValue('pat-secret-once')).toHaveLength(1);
  });

  it('keeps the create dialog open and shows an inline creation failure', async () => {
    mockedCreateToken.mockRejectedValue(new Error('unavailable'));
    render(<AccessTokenView />);

    await submitCreateDialog();

    expect(
      await screen.findByText('Unable to create the access token. Please try again.'),
    ).toBeTruthy();
    expect(screen.getByRole('dialog', { name: 'Add access token' })).toBeTruthy();
    expect(screen.queryByLabelText('One-time personal access token')).toBeNull();
  });

  it('confirms rotation, disables token actions, shows the replacement once, and refreshes metadata', async () => {
    let resolveRotate: ((value: any) => void) | undefined;
    mockedGetTokens
      .mockResolvedValueOnce(
        page([{ ...summary, token: 'old-token-must-not-render' } as API.UserAuthTokenSummary]),
      )
      .mockResolvedValueOnce(page([{ ...summary, fingerprint: 'sha256:rotated' }]));
    mockedRotateToken.mockReturnValue(
      new Promise((resolve) => {
        resolveRotate = resolve;
      }) as any,
    );
    render(<AccessTokenView />);

    const rotateButton = await screen.findByRole('button', { name: 'Rotate pat-1' });
    const revokeButton = screen.getByRole('button', { name: 'Revoke pat-1' });
    fireEvent.click(rotateButton);
    expect(await screen.findByText('Rotate this access token?')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Confirm rotate' }));

    await waitFor(() => {
      expect(mockedRotateToken).toHaveBeenCalledWith(
        { id: 'pat-1' },
        { skipErrorHandler: true },
      );
    });
    expect((rotateButton as HTMLButtonElement).disabled).toBe(true);
    expect((revokeButton as HTMLButtonElement).disabled).toBe(true);
    expect(
      (screen.getByRole('button', { name: 'Add access token' }) as HTMLButtonElement).disabled,
    ).toBe(true);
    expect(screen.getByText('Rotating')).toBeTruthy();

    await act(async () => {
      resolveRotate?.({
        ...summary,
        fingerprint: 'sha256:rotated',
        token: 'pat-rotated-secret-once',
      });
    });

    const secretInput = await screen.findByLabelText('One-time personal access token');
    expect((secretInput as HTMLTextAreaElement).value).toBe('pat-rotated-secret-once');
    expect(writeText).not.toHaveBeenCalled();
    expect(screen.queryByText('old-token-must-not-render')).toBeNull();
    expect(screen.queryByDisplayValue('old-token-must-not-render')).toBeNull();
    expect(await screen.findByText(/sha256:rotated/)).toBeTruthy();
    expect(mockedGetTokens).toHaveBeenCalledTimes(2);

    fireEvent.click(screen.getByRole('button', { name: 'I have saved the token' }));
    await waitFor(() => {
      expect(screen.queryByDisplayValue('pat-rotated-secret-once')).toBeNull();
    });
  });

  it('keeps metadata visible and does not open the secret dialog when rotation fails', async () => {
    mockedGetTokens.mockResolvedValue(page([summary]));
    mockedRotateToken.mockRejectedValue(new Error('conflict'));
    render(<AccessTokenView />);

    fireEvent.click(await screen.findByRole('button', { name: 'Rotate pat-1' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm rotate' }));

    expect(
      await screen.findByText('Unable to rotate the access token. Please try again.'),
    ).toBeTruthy();
    expect(screen.getByText(/pat-1/)).toBeTruthy();
    expect(screen.queryByLabelText('One-time personal access token')).toBeNull();
  });

  it('treats a rotation response without a one-time token as a failure', async () => {
    mockedGetTokens.mockResolvedValue(page([summary]));
    mockedRotateToken.mockResolvedValue({ ...summary, fingerprint: 'sha256:rotated' });
    render(<AccessTokenView />);

    fireEvent.click(await screen.findByRole('button', { name: 'Rotate pat-1' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm rotate' }));

    expect(
      await screen.findByText('Unable to rotate the access token. Please try again.'),
    ).toBeTruthy();
    expect(screen.queryByLabelText('One-time personal access token')).toBeNull();
    expect(mockedGetTokens).toHaveBeenCalledTimes(1);
  });

  it('requires confirmation, shows revoke progress, and refreshes the list', async () => {
    let resolveRevoke: ((value: any) => void) | undefined;
    mockedGetTokens.mockResolvedValueOnce(page([summary])).mockResolvedValueOnce(page([]));
    mockedRevokeToken.mockReturnValue(
      new Promise((resolve) => {
        resolveRevoke = resolve;
      }) as any,
    );
    render(<AccessTokenView />);

    const revokeButton = await screen.findByRole('button', { name: 'Revoke pat-1' });
    fireEvent.click(revokeButton);
    expect(await screen.findByText('Revoke this access token?')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Confirm revoke' }));

    await waitFor(() => {
      expect(mockedRevokeToken).toHaveBeenCalledWith({ id: 'pat-1' }, { skipErrorHandler: true });
    });
    expect((revokeButton as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByText('Revoking')).toBeTruthy();

    await act(async () => {
      resolveRevoke?.({});
    });

    expect(await screen.findByText('Access token revoked.')).toBeTruthy();
    expect(await screen.findByText('No access tokens')).toBeTruthy();
  });

  it('keeps the summary visible when revocation fails', async () => {
    mockedGetTokens.mockResolvedValue(page([summary]));
    mockedRevokeToken.mockRejectedValue(new Error('conflict'));
    render(<AccessTokenView />);

    fireEvent.click(await screen.findByRole('button', { name: 'Revoke pat-1' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Confirm revoke' }));

    expect(
      await screen.findByText('Unable to revoke the access token. Please try again.'),
    ).toBeTruthy();
    expect(screen.getByText(/pat-1/)).toBeTruthy();
  });
});
