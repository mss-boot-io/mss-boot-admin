import { runWithOneTimeCredential } from './credentialLifecycle';

describe('runWithOneTimeCredential', () => {
  it('clears a sent credential before asking the user to reauthorize on failure', async () => {
    const events: string[] = [];
    const failure = new Error('generation failed after consume');

    await expect(
      runWithOneTimeCredential({
        credential: 'opaque-handle',
        request: async (credential) => {
          expect(credential).toBe('opaque-handle');
          throw failure;
        },
        clearCredential: () => events.push('clear'),
        onCredentialMissing: () => events.push('missing'),
        onCredentialFailure: () => events.push('reauthorize'),
      }),
    ).rejects.toBe(failure);
    expect(events).toEqual(['clear', 'reauthorize']);
  });

  it('clears a sent credential after successful generation without a failure prompt', async () => {
    const clearCredential = jest.fn();
    const onCredentialFailure = jest.fn();

    await expect(
      runWithOneTimeCredential({
        credential: 'opaque-handle',
        request: async (credential) => {
          expect(credential).toBe('opaque-handle');
          return { repo: 'example', branch: 'generated' };
        },
        clearCredential,
        onCredentialMissing: jest.fn(),
        onCredentialFailure,
      }),
    ).resolves.toEqual({ repo: 'example', branch: 'generated' });
    expect(clearCredential).toHaveBeenCalledTimes(1);
    expect(onCredentialFailure).not.toHaveBeenCalled();
  });

  it('does not send a generation request without a credential', async () => {
    const request = jest.fn();
    const clearCredential = jest.fn();
    const onCredentialMissing = jest.fn();
    const onCredentialFailure = jest.fn();

    await expect(
      runWithOneTimeCredential({
        request,
        clearCredential,
        onCredentialMissing,
        onCredentialFailure,
      }),
    ).resolves.toBeUndefined();
    expect(request).not.toHaveBeenCalled();
    expect(clearCredential).not.toHaveBeenCalled();
    expect(onCredentialMissing).toHaveBeenCalledTimes(1);
    expect(onCredentialFailure).not.toHaveBeenCalled();
  });
});
