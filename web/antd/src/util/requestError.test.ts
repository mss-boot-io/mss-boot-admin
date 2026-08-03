import { getResponseErrorMessage } from './requestError';

describe('getResponseErrorMessage', () => {
  it('uses backend msg fields when present', () => {
    expect(
      getResponseErrorMessage({
        status: 400,
        statusText: 'Bad Request',
        data: { msg: 'invalid config' },
      }),
    ).toBe('Bad Request: invalid config');
  });

  it('supports plain text response bodies', () => {
    expect(
      getResponseErrorMessage({
        status: 502,
        statusText: 'Bad Gateway',
        data: 'upstream unavailable',
      }),
    ).toBe('Bad Gateway: upstream unavailable');
  });

  it('falls back to status code without throwing for empty bodies', () => {
    expect(
      getResponseErrorMessage({
        status: 500,
        data: undefined,
      }),
    ).toBe('HTTP 500');
  });
});
