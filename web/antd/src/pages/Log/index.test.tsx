import { canReadRuntimeLogs } from './index';

describe('runtime log access', () => {
  it('keeps the runtime tab for root users', () => {
    expect(canReadRuntimeLogs({ role: { root: true } })).toBe(true);
  });

  it('shows the runtime tab only for the exact component permission', () => {
    expect(canReadRuntimeLogs({ permissions: { '/log/runtime': true } })).toBe(true);
    expect(canReadRuntimeLogs({ permissions: { '/log/runtime': false } })).toBe(false);
    expect(canReadRuntimeLogs({ permissions: { '/log/runtime': 'component' } })).toBe(false);
    expect(canReadRuntimeLogs({ permissions: { '/log': true } })).toBe(false);
    expect(canReadRuntimeLogs({ permissions: { '/log/runtime/export': true } })).toBe(false);
  });

  it('hides the runtime tab while identity permissions are unavailable', () => {
    expect(canReadRuntimeLogs()).toBe(false);
    expect(canReadRuntimeLogs({})).toBe(false);
  });
});
