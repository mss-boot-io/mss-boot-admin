import { describe, expect, it } from 'vitest';
import {
  parseEmailAppConfig,
  parseSecurityAppConfig,
  serializeEmailAppConfig,
  serializeSecurityAppConfig,
} from './contracts';

describe('application config contracts', () => {
  it('strips all V6 protected secrets from query-owned data', () => {
    const security = parseSecurityAppConfig({
      githubBrowserSessionClientSecret: 'browser-github-secret',
      larkBrowserSessionAppSecret: 'browser-lark-secret',
    });
    const email = parseEmailAppConfig({ password: 'smtp-secret', smtpPort: '587' });

    expect(security.configuredSecrets).toEqual(
      new Set(['githubBrowserSessionClientSecret', 'larkBrowserSessionAppSecret']),
    );
    expect(security.values).not.toHaveProperty('githubBrowserSessionClientSecret');
    expect(security.values).not.toHaveProperty('larkBrowserSessionAppSecret');
    expect(email.configuredSecrets).toEqual(new Set(['password']));
    expect(email.values).not.toHaveProperty('password');
  });

  it('ignores retired OAuth fields and keeps only V6 browser-session values', () => {
    const parsed = parseSecurityAppConfig({
      githubClientId: 'legacy-id',
      githubBrowserSessionClientId: 'browser-id',
      githubRedirectURI: 'https://legacy.example/callback',
      githubBrowserSessionRedirectURI: 'https://v6.example/callback',
    });
    expect(parsed.values).toMatchObject({
      githubBrowserSessionClientId: 'browser-id',
      githubBrowserSessionRedirectURI: 'https://v6.example/callback',
    });
    expect(parsed.values).not.toHaveProperty('githubClientId');
    expect(parsed.values).not.toHaveProperty('githubRedirectURI');
  });

  it('omits blank or unauthorized secret rotations from writes', () => {
    const values = parseSecurityAppConfig({}).values;
    values.githubBrowserSessionClientSecret = '  rotate-me  ';
    values.larkBrowserSessionAppSecret = '   ';
    expect(serializeSecurityAppConfig(values, true)).toMatchObject({
      githubBrowserSessionClientSecret: 'rotate-me',
    });
    expect(serializeSecurityAppConfig(values, true)).not.toHaveProperty(
      'larkBrowserSessionAppSecret',
    );
    expect(serializeSecurityAppConfig(values, false)).not.toHaveProperty(
      'githubBrowserSessionClientSecret',
    );
    expect(
      serializeEmailAppConfig(
        { smtpHost: 'smtp.example', smtpPort: 587, username: 'mailer', password: ' secret ' },
        true,
      ),
    ).toMatchObject({ password: 'secret' });
  });
});
