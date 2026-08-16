export interface SessionDevice {
  browser?: string;
  operatingSystem?: string;
  mobile: boolean;
}

/** Parse only presentation-safe product and platform names; retain the raw UA in details. */
export function identifySessionDevice(userAgent?: string): SessionDevice {
  const value = userAgent?.trim() ?? '';
  const browser = (() => {
    if (/EdgA?\//.test(value)) return 'Microsoft Edge';
    if (/OPR\//.test(value)) return 'Opera';
    if (/(?:Chrome|CriOS)\//.test(value)) return 'Google Chrome';
    if (/(?:Firefox|FxiOS)\//.test(value)) return 'Mozilla Firefox';
    if (/Safari\//.test(value) && /Version\//.test(value)) return 'Safari';
    if (/curl\//i.test(value)) return 'curl';
    return undefined;
  })();
  const operatingSystem = (() => {
    if (/Windows NT/.test(value)) return 'Windows';
    if (/Android/.test(value)) return 'Android';
    if (/iPad/.test(value)) return 'iPadOS';
    if (/(?:iPhone|iPod)/.test(value)) return 'iOS';
    if (/Mac OS X/.test(value)) return 'macOS';
    if (/Linux/.test(value)) return 'Linux';
    return undefined;
  })();
  return {
    browser,
    operatingSystem,
    mobile: /Mobile|Android|iPhone|iPad|iPod/i.test(value),
  };
}

export function sessionDeviceSummary(
  userAgent: string | undefined,
  mobileLabel: string,
  unknownLabel: string,
): string {
  const device = identifySessionDevice(userAgent);
  const values = [
    device.browser,
    device.operatingSystem,
    device.mobile ? mobileLabel : undefined,
  ].filter((value): value is string => Boolean(value));
  return values.length > 0 ? values.join(' · ') : unknownLabel;
}
