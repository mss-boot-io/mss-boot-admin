import { getUserDisplayName } from './userDisplayName';

describe('getUserDisplayName', () => {
  it('prefers a non-blank display name', () => {
    expect(getUserDisplayName({ name: '  Ada Lovelace  ', username: 'ada' })).toBe('Ada Lovelace');
  });

  it('falls back to the username when the display name is blank', () => {
    expect(getUserDisplayName({ name: '   ', username: 'admin' })).toBe('admin');
  });

  it('returns an empty string only when neither display name nor username is available', () => {
    expect(getUserDisplayName({ name: '', username: '   ' })).toBe('');
    expect(getUserDisplayName()).toBe('');
  });
});
