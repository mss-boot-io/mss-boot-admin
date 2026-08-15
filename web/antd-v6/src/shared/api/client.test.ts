import { describe, expect, it } from 'vitest';
import { browserRequestHeaders } from './client';

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
});
