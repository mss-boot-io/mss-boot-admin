const mockGetIntl = jest.fn(() => ({
  formatMessage: ({ id }: { id: string }) => `i18n:${id}`,
}));
const mockMessageError = jest.fn();
const mockMessageWarning = jest.fn();
const mockNotificationDestroy = jest.fn();
const mockNotificationOpen = jest.fn();

jest.mock('@umijs/max', () => ({
  getIntl: mockGetIntl,
}));

jest.mock('antd', () => ({
  Button: 'button',
  message: { error: mockMessageError, warning: mockMessageWarning },
  notification: {
    destroy: mockNotificationDestroy,
    open: mockNotificationOpen,
  },
}));

jest.mock('../config/defaultSettings', () => ({
  __esModule: true,
  default: { pwa: true },
}));

describe('PWA global events', () => {
  const listeners = new Map<string, EventListener>();
  let addEventListenerSpy: jest.SpyInstance;
  let globalModule: typeof import('./global');

  beforeEach(() => {
    jest.clearAllMocks();
    jest.resetModules();
    listeners.clear();
    addEventListenerSpy = jest.spyOn(window, 'addEventListener').mockImplementation(
      ((type: string, listener: EventListenerOrEventListenerObject) => {
        if (typeof listener === 'function') listeners.set(type, listener);
      }) as typeof window.addEventListener,
    );

    jest.isolateModules(() => {
      globalModule = require('./global');
    });
  });

  afterEach(() => {
    addEventListenerSpy.mockRestore();
  });

  it('formats the offline warning through the imperative locale API', () => {
    listeners.get('sw.offline')?.(new Event('sw.offline'));

    expect(mockGetIntl).toHaveBeenCalledTimes(1);
    expect(mockMessageWarning).toHaveBeenCalledWith('i18n:app.pwa.offline');
  });

  it('formats every service-worker update prompt through the imperative locale API', () => {
    listeners.get('sw.updated')?.(new CustomEvent('sw.updated'));

    expect(mockNotificationOpen).toHaveBeenCalledTimes(1);
    expect(mockNotificationOpen).toHaveBeenCalledWith(
      expect.objectContaining({
        description: 'i18n:app.pwa.serviceworker.updated.hint',
        message: 'i18n:app.pwa.serviceworker.updated',
      }),
    );
    const notification = mockNotificationOpen.mock.calls[0][0];
    expect(notification.btn.props.children).toBe('i18n:app.pwa.serviceworker.updated.ok');
  });

  it('waits for skip-waiting before reloading without deleting the new Workbox precache', async () => {
    const order: string[] = [];
    let channel: {
      port1: { onmessage?: (event: { data: Record<string, unknown> }) => void };
      port2: Record<string, never>;
    };
    const OriginalMessageChannel = global.MessageChannel;
    class TestMessageChannel {
      port1: { onmessage?: (event: { data: Record<string, unknown> }) => void } = {};
      port2 = {};

      constructor() {
        channel = { port1: this.port1, port2: this.port2 };
      }
    }
    Object.defineProperty(global, 'MessageChannel', {
      configurable: true,
      value: TestMessageChannel,
    });
    const cacheKeys = jest.fn(async () => ['antd-pro-precache-v5']);
    const cacheDelete = jest.fn(async () => true);
    Object.defineProperty(window, 'caches', {
      configurable: true,
      value: {
        keys: cacheKeys,
        delete: cacheDelete,
      },
    });
    const worker = {
      postMessage: jest.fn(() => {
        order.push('skip-waiting');
        channel.port1.onmessage?.({ data: { ok: true } });
      }),
    };

    await globalModule.applyServiceWorkerUpdate(worker as unknown as ServiceWorker, () => {
      order.push('reload');
    });

    expect(order).toEqual(['skip-waiting', 'reload']);
    expect(cacheKeys).not.toHaveBeenCalled();
    expect(cacheDelete).not.toHaveBeenCalled();
    Object.defineProperty(global, 'MessageChannel', {
      configurable: true,
      value: OriginalMessageChannel,
    });
    Reflect.deleteProperty(window, 'caches');
  });

  it('does not reload when skip-waiting fails', async () => {
    let channel: {
      port1: { onmessage?: (event: { data: Record<string, unknown> }) => void };
      port2: Record<string, never>;
    };
    const OriginalMessageChannel = global.MessageChannel;
    class TestMessageChannel {
      port1: { onmessage?: (event: { data: Record<string, unknown> }) => void } = {};
      port2 = {};

      constructor() {
        channel = { port1: this.port1, port2: this.port2 };
      }
    }
    Object.defineProperty(global, 'MessageChannel', {
      configurable: true,
      value: TestMessageChannel,
    });
    const cacheKeys = jest.fn(async () => []);
    Object.defineProperty(window, 'caches', {
      configurable: true,
      value: { keys: cacheKeys, delete: jest.fn() },
    });
    const reload = jest.fn();
    const worker = {
      postMessage: jest.fn(() => {
        channel.port1.onmessage?.({ data: { error: 'activation failed' } });
      }),
    };

    await expect(
      globalModule.applyServiceWorkerUpdate(worker as unknown as ServiceWorker, reload),
    ).rejects.toBe('activation failed');
    expect(cacheKeys).not.toHaveBeenCalled();
    expect(reload).not.toHaveBeenCalled();
    Object.defineProperty(global, 'MessageChannel', {
      configurable: true,
      value: OriginalMessageChannel,
    });
    Reflect.deleteProperty(window, 'caches');
  });
});
