import { describe, expect, it } from 'vitest';
import { identifySessionDevice, sessionDeviceSummary } from './device';

describe('session device presentation', () => {
  it('identifies common desktop and mobile user agents without exposing the raw value', () => {
    expect(
      identifySessionDevice(
        'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/140.0 Safari/537.36',
      ),
    ).toEqual({ browser: 'Google Chrome', operatingSystem: 'Windows', mobile: false });

    expect(
      sessionDeviceSummary(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1 Version/19.0 Mobile/15E148 Safari/604.1',
        'Mobile',
        'Unknown',
      ),
    ).toBe('Safari · iOS · Mobile');
  });

  it('uses a product-safe fallback for unknown or absent user agents', () => {
    expect(sessionDeviceSummary(undefined, 'Mobile', 'Unknown device')).toBe('Unknown device');
  });
});
