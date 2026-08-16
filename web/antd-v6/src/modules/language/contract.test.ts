import { describe, expect, it } from 'vitest';
import {
  LanguageContractError,
  parseLanguageDetail,
  parseLanguagePage,
  parseLanguageProfile,
  serializeLanguageWrite,
} from './contract';

const summary = {
  id: 'language-1',
  name: 'en-US',
  remark: '',
  status: 'enabled',
  updatedAt: '2026-08-15T00:00:00Z',
};

describe('language contract', () => {
  it('parses a bounded summary page and full detail independently', () => {
    expect(
      parseLanguagePage(
        { data: [summary], total: 1, current: 1, pageSize: 20 },
        { current: 1, pageSize: 20 },
      ),
    ).toMatchObject({ total: 1, data: [{ name: 'en-US' }] });

    expect(
      parseLanguageDetail({
        ...summary,
        defines: [{ id: 'welcome', group: 'menu', key: 'welcome', value: 'Welcome' }],
      }).defines,
    ).toEqual([{ id: 'welcome', group: 'menu', key: 'welcome', value: 'Welcome' }]);
  });

  it('rejects malformed pagination, duplicate keys, and missing server IDs', () => {
    expect(() =>
      parseLanguagePage(
        { data: [summary], total: 1, current: 2, pageSize: 20 },
        { current: 1, pageSize: 20 },
      ),
    ).toThrow(LanguageContractError);
    expect(() =>
      parseLanguageDetail({
        ...summary,
        defines: [
          { id: 'a', group: 'menu', key: 'welcome', value: 'A' },
          { id: 'b', group: 'menu', key: 'welcome', value: 'B' },
        ],
      }),
    ).toThrow(/unique/);
    expect(() =>
      parseLanguageDetail({
        ...summary,
        defines: [{ group: 'menu', key: 'welcome', value: 'Welcome' }],
      }),
    ).toThrow(/missing/);
  });

  it('serializes only the write allowlist and preserves translation whitespace', () => {
    const source = {
      id: 'client-language-id',
      createdAt: '2000-01-01T00:00:00Z',
      name: ' en-US ',
      remark: ' note ',
      status: 'enabled' as const,
      defines: [{ id: '', group: ' menu ', key: 'welcome', value: ' Welcome ' }],
    };
    expect(serializeLanguageWrite(source, '2026-08-15T00:00:00Z')).toEqual({
      name: 'en-US',
      remark: 'note',
      status: 'enabled',
      defines: [{ group: 'menu', key: 'welcome', value: ' Welcome ' }],
      expectedUpdatedAt: '2026-08-15T00:00:00Z',
    });
  });

  it('accepts full BCP 47 locale forms and canonicalizes their casing', () => {
    expect(
      serializeLanguageWrite({
        name: 'zh-hans-cn',
        status: 'enabled',
        defines: [],
      }).name,
    ).toBe('zh-Hans-CN');
    expect(() =>
      serializeLanguageWrite({ name: 'not_a_locale', status: 'enabled', defines: [] }),
    ).toThrow(LanguageContractError);
  });

  it('sends definition identities only when updating an exact revision', () => {
    const values = {
      name: 'en-US',
      status: 'enabled' as const,
      defines: [{ id: 'server-owned', group: 'menu', key: 'welcome', value: 'Welcome' }],
    };
    expect(serializeLanguageWrite(values).defines).toEqual([
      { group: 'menu', key: 'welcome', value: 'Welcome' },
    ]);
    expect(serializeLanguageWrite(values, '2026-08-15T00:00:00Z').defines).toEqual([
      { id: 'server-owned', group: 'menu', key: 'welcome', value: 'Welcome' },
    ]);
  });

  it('projects only fully supported runtime locales', () => {
    const profile = parseLanguageProfile({
      'en-US': { 'menu.language': 'Languages' },
      'fr-FR': { 'menu.language': 'Langues' },
    });
    expect(profile).toEqual({ 'en-US': { 'menu.language': 'Languages' } });
    expect(Object.isFrozen(profile['en-US'])).toBe(true);
  });
});
