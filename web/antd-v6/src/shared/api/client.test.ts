import { describe, expect, it } from 'vitest';
import {
  browserRequestHeaders,
  getRequestErrorCode,
  getRequestErrorMessage,
  getRequestStatus,
} from './client';

describe('browser request security headers', () => {
  it('copies the readable signed CSRF cookie only to unsafe requests', () => {
    const mutationHeaders = new Headers(
      browserRequestHeaders('POST', { 'If-Match': '"revision-4"' }, 'signed-csrf-value'),
    );
    expect(mutationHeaders.get('Accept')).toBe('application/json');
    expect(mutationHeaders.get('If-Match')).toBe('"revision-4"');
    expect(mutationHeaders.get('X-CSRF-Token')).toBe('signed-csrf-value');
    expect(
      new Headers(browserRequestHeaders('GET', undefined, 'signed-csrf-value')).get('X-CSRF-Token'),
    ).toBeNull();
  });

  it('does not synthesize a bearer credential', () => {
    expect(
      new Headers(browserRequestHeaders('DELETE', undefined, 'signed-csrf-value')).get(
        'Authorization',
      ),
    ).toBeNull();
  });

  it('classifies transport failures without flattening their status', () => {
    const failure = Object.assign(new Error('fallback'), {
      response: { status: 503, data: { errorMessage: 'policy unavailable' } },
    });
    expect(getRequestStatus(failure)).toBe(503);
    expect(getRequestErrorMessage(failure)).toBe('policy unavailable');
  });

  it('reads stable business errors from the Umi request error body', () => {
    const failure = Object.assign(new Error('fallback'), {
      data: {
        errorCode: 'SECURITY_SESSION_REQUIRED',
        errorMessage: 'an interactive server session is required',
      },
      response: { status: 401 },
    });
    expect(getRequestStatus(failure)).toBe(401);
    expect(getRequestErrorCode(failure)).toBe('SECURITY_SESSION_REQUIRED');
    expect(getRequestErrorMessage(failure)).toBe('an interactive server session is required');
  });

  it('preserves an explicit versioned media type', () => {
    const headers = browserRequestHeaders('GET', {
      Accept: 'application/vnd.mss.theme.v1+json',
    });
    expect(new Headers(headers).get('Accept')).toBe('application/vnd.mss.theme.v1+json');
  });
});
