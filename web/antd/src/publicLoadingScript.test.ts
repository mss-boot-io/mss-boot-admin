const fs = require('node:fs');
const path = require('node:path');

const loadingScript = fs.readFileSync(
  path.join(process.cwd(), 'public/scripts/loading.js'),
  'utf8',
);

const executeLoadingScript = ({
  getRoot,
  locale = 'en-US',
  onObserve,
}: {
  getRoot: () => { innerHTML: string } | null;
  locale?: string;
  onObserve?: (callback: () => void) => void;
}) => {
  const documentStub = {
    addEventListener: jest.fn(),
    documentElement: { lang: 'en-US' },
    querySelector: jest.fn(getRoot),
    readyState: 'loading',
  };
  class ObserverStub {
    callback: () => void;

    constructor(callback: () => void) {
      this.callback = callback;
      onObserve?.(callback);
    }

    disconnect = jest.fn();

    observe = jest.fn();
  }

  new Function('document', 'navigator', 'localStorage', 'MutationObserver', loadingScript)(
    documentStub,
    { language: 'en-US' },
    { getItem: () => locale },
    ObserverStub,
  );

  return documentStub;
};

describe('public loading placeholder', () => {
  it('renders immediately when the root already exists during HTML parsing', () => {
    const root = { innerHTML: '' };

    const documentStub = executeLoadingScript({ getRoot: () => root });

    expect(root.innerHTML).toContain('role="status"');
    expect(root.innerHTML).toContain('Loading application');
    expect(documentStub.addEventListener).not.toHaveBeenCalled();
  });

  it('observes for the root instead of waiting for the application bundle', () => {
    let root: { innerHTML: string } | null = null;
    let observerCallback: (() => void) | undefined;

    executeLoadingScript({
      getRoot: () => root,
      locale: 'zh-CN',
      onObserve: (callback) => {
        observerCallback = callback;
      },
    });

    root = { innerHTML: '' };
    observerCallback?.();

    expect(root.innerHTML).toContain('正在加载资源');
  });
});
