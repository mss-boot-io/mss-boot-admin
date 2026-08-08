import { getRoleActionDisabledState } from './roleActions';

describe('managed role actions', () => {
  it('disables every lifecycle action for the root role', () => {
    expect(getRoleActionDisabledState({ root: true, default: false })).toEqual({
      edit: true,
      delete: true,
      authorize: true,
    });
  });

  it('keeps authorization available while protecting the default role lifecycle', () => {
    expect(getRoleActionDisabledState({ root: false, default: true })).toEqual({
      edit: true,
      delete: true,
      authorize: false,
    });
  });

  it('keeps ordinary role actions enabled', () => {
    expect(getRoleActionDisabledState({ root: false, default: false })).toEqual({
      edit: false,
      delete: false,
      authorize: false,
    });
  });
});
