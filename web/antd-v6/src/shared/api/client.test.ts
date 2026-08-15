import { describe, expect, it } from 'vitest';
import { browserRequestHeaders, getRequestErrorMessage, getRequestStatus } from './client';

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

  it('preserves an explicit versioned media type', () => {
    const headers = browserRequestHeaders('GET', {
      Accept: 'application/vnd.mss.theme.v1+json',
    });
    expect(new Headers(headers).get('Accept')).toBe('application/vnd.mss.theme.v1+json');
  });
});
