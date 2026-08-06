import { request } from '@umijs/max';
import {
  OAUTH_CREDENTIAL_HEADER,
  getTemplateGetBranches,
  getTemplateGetParams,
  getTemplateGetPath,
  postTemplateGenerate,
} from './generator';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
}));

const mockRequest = request as jest.Mock;

describe('generator OAuth credential contract', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockRequest.mockResolvedValue({});
  });

  it('sends the opaque handle only through X-MSS-OAuth-Credential', async () => {
    const handle = 'opaque-credential-handle';

    await getTemplateGetBranches({ source: 'github.com/org/template' }, handle);
    await getTemplateGetPath(
      { source: 'github.com/org/template', branch: 'main' },
      handle,
    );
    await getTemplateGetParams(
      { source: 'github.com/org/template', branch: 'main', path: 'service' },
      handle,
    );
    await postTemplateGenerate(
      {
        email: 'dev@example.com',
        template: { source: 'github.com/org/template', branch: 'main', path: 'service' },
        generate: { repo: 'example', service: 'api' },
      },
      handle,
    );

    expect(mockRequest).toHaveBeenCalledTimes(4);
    mockRequest.mock.calls.forEach(([url, config]) => {
      expect(url).not.toContain(handle);
      expect(config.headers[OAUTH_CREDENTIAL_HEADER]).toBe(handle);
      expect(JSON.stringify(config.params || {})).not.toContain(handle);
      expect(JSON.stringify(config.data || {})).not.toContain(handle);
      expect(JSON.stringify(config.params || {})).not.toContain('accessToken');
      expect(JSON.stringify(config.data || {})).not.toContain('accessToken');
    });
  });

  it('keeps public-template requests credential-free', async () => {
    await getTemplateGetBranches({ source: 'github.com/public/template' });
    await getTemplateGetPath({
      source: 'github.com/public/template',
      branch: 'main',
    });
    await getTemplateGetParams({
      source: 'github.com/public/template',
      branch: 'main',
      path: 'service',
    });

    expect(mockRequest).toHaveBeenCalledTimes(3);
    mockRequest.mock.calls.forEach(([, config]) => {
      expect(config.method).toBe('GET');
      expect(config.headers).toBeUndefined();
    });
  });
});
