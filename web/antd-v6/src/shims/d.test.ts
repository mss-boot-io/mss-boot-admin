import { createRequire } from 'node:module';
import { describe, expect, it, vi } from 'vitest';

interface DescriptorFactory {
  (value: unknown, options?: PropertyDescriptor): PropertyDescriptor;
  (specification: string, value: unknown, options?: PropertyDescriptor): PropertyDescriptor;
  gs: (
    specification: string | null,
    get?: (() => unknown) | null,
    set?: ((value: unknown) => void) | null,
    options?: PropertyDescriptor,
  ) => PropertyDescriptor;
}

const require = createRequire(import.meta.url);
const descriptor = require('./d.cjs') as DescriptorFactory;

describe('modern d compatibility shim', () => {
  it('preserves data descriptor flags and option precedence', () => {
    expect(descriptor('cew', 3)).toEqual({
      value: 3,
      configurable: true,
      enumerable: true,
      writable: true,
    });
    expect(descriptor('w', undefined, { enumerable: true })).toEqual({
      value: undefined,
      configurable: false,
      enumerable: false,
      writable: true,
    });
  });

  it('preserves getter, setter, and set-only argument forms', () => {
    const get = vi.fn(() => 'value');
    const set = vi.fn();
    expect(descriptor.gs('ce', get, set)).toEqual({
      get,
      set,
      configurable: true,
      enumerable: true,
    });

    const setOnly = vi.fn();
    const target = {} as { value?: unknown };
    Object.defineProperty(target, 'value', descriptor.gs(null, setOnly));
    target.value = 7;
    expect(setOnly).toHaveBeenCalledWith(7);
  });
});
