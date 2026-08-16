import { describe, expect, it } from 'vitest';
import {
  parseEmailAppConfig,
  parseSecurityAppConfig,
  serializeEmailAppConfig,
  serializeSecurityAppConfig,
} from './contracts';

describe('application config contracts', () => {
  it('strips all five protected secrets from query-owned data', () => {
    const security = parseSecurityAppConfig({
      githubClientSecret: 'legacy-github-secret',
      githubBrowserSessionClientSecret: 'browser-github-secret',
      larkAppSecret: 'legacy-lark-secret',
      larkBrowserSessionAppSecret: 'browser-lark-secret',
    });
    const email = parseEmailAppConfig({ password: 'smtp-secret', smtpPort: '587' });

    expect(security.configuredSecrets).toEqual(
      new Set([
        'githubClientSecret',
        'githubBrowserSessionClientSecret',
        'larkAppSecret',
        'larkBrowserSessionAppSecret',
      ]),
    );
    expect(security.values).not.toHaveProperty('githubClientSecret');
    expect(security.values).not.toHaveProperty('larkBrowserSessionAppSecret');
    expect(email.configuredSecrets).toEqual(new Set(['password']));
    expect(email.values).not.toHaveProperty('password');
  });

  it('keeps V5 and V6 OAuth credential fields independent', () => {
    const parsed = parseSecurityAppConfig({
      githubClientId: 'legacy-id',
      githubBrowserSessionClientId: 'browser-id',
      githubRedirectURI: 'https://legacy.example/callback',
      githubBrowserSessionRedirectURI: 'https://v6.example/callback',
    });
    expect(parsed.values).toMatchObject({
      githubClientId: 'legacy-id',
      githubBrowserSessionClientId: 'browser-id',
      githubRedirectURI: 'https://legacy.example/callback',
      githubBrowserSessionRedirectURI: 'https://v6.example/callback',
    });
  });

  it('omits blank or unauthorized secret rotations from writes', () => {
    const values = parseSecurityAppConfig({}).values;
    values.githubClientSecret = '  rotate-me  ';
    values.githubBrowserSessionClientSecret = '   ';
    expect(serializeSecurityAppConfig(values, true)).toMatchObject({
      githubClientSecret: 'rotate-me',
    });
    expect(serializeSecurityAppConfig(values, true)).not.toHaveProperty(
      'githubBrowserSessionClientSecret',
    );
    expect(serializeSecurityAppConfig(values, false)).not.toHaveProperty('githubClientSecret');
    expect(
      serializeEmailAppConfig(
        { smtpHost: 'smtp.example', smtpPort: 587, username: 'mailer', password: ' secret ' },
        true,
      ),
    ).toMatchObject({ password: 'secret' });
  });
});
