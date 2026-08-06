import { getMenuLocaleId } from './menuLocale';

describe('getMenuLocaleId', () => {
  it('keeps a direct locale key unchanged', () => {
    expect(getMenuLocaleId('menu.origination.user')).toBe('menu.origination.user');
  });

  it('removes a hierarchy prefix added by the menu tree response', () => {
    expect(getMenuLocaleId('menu.origination.menu.origination.user')).toBe('menu.origination.user');
  });

  it('adds the locale namespace to a plain menu name', () => {
    expect(getMenuLocaleId('welcome')).toBe('menu.welcome');
  });

  it('returns an empty id for a blank menu name', () => {
    expect(getMenuLocaleId('  ')).toBe('');
  });
});
