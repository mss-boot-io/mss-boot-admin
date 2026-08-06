type OneTimeCredentialRequest<T> = {
  credential?: string;
  request: (credential: string) => Promise<T>;
  clearCredential: () => void;
  onCredentialMissing: () => void;
  onCredentialFailure: () => void;
};

/**
 * A generator OAuth handle is consumed by the backend before side effects.
 * Generation is never attempted without a handle. Once sent, the browser must
 * forget it even when generation fails.
 */
export async function runWithOneTimeCredential<T>({
  credential,
  request,
  clearCredential,
  onCredentialMissing,
  onCredentialFailure,
}: OneTimeCredentialRequest<T>): Promise<T | undefined> {
  if (!credential) {
    onCredentialMissing();
    return undefined;
  }

  let failed = false;
  try {
    return await request(credential);
  } catch (error) {
    failed = true;
    throw error;
  } finally {
    clearCredential();
    if (failed) {
      onCredentialFailure();
    }
  }
}
