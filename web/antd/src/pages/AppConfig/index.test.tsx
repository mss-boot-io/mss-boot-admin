import * as React from 'react';
import { render, screen } from '@testing-library/react';
import Settings from './index';

const mockSetSearchParams = jest.fn();
const mockTabs = jest.fn();
let mockIsMobile = false;
let mockSearchKey: string | null = null;

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ id, defaultMessage }: { id: string; defaultMessage?: string }) =>
      defaultMessage || id,
  }),
  useSearchParams: () => [
    { get: (name: string) => (name === 'key' ? mockSearchKey : null) },
    mockSetSearchParams,
  ],
}));

jest.mock('@/hooks/useResponsive', () => ({
  useResponsive: () => ({ isMobile: mockIsMobile }),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children, title }: any) => (
    <section data-testid="page-container" data-title={String(title)}>
      {children}
    </section>
  ),
}));

jest.mock('antd', () => ({
  Tabs: (props: any) => {
    mockTabs(props);
    const activeItem = props.items?.find((item: { key: string }) => item.key === props.activeKey);
    return (
      <div data-testid="tabs" data-tab-position={props.tabPosition} style={props.style}>
        {activeItem?.children}
      </div>
    );
  },
}));

jest.mock('./components/base', () => () => <div>base-panel</div>);
jest.mock('./components/security', () => () => <div>security-panel</div>);
jest.mock('./components/storage', () => () => <div>storage-panel</div>);
jest.mock('./components/email', () => () => <div>email-panel</div>);
jest.mock('../../components/MssBoot/theme', () => () => <div>theme-panel</div>);

describe('AppConfig settings tabs', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockIsMobile = false;
    mockSearchKey = null;
  });

  it('uses top, full-width tabs at the 375px mobile breakpoint and falls back to base', () => {
    mockIsMobile = true;
    mockSearchKey = 'unknown';

    render(<Settings />);

    expect(screen.getByTestId('page-container').getAttribute('data-title')).toBe(
      'Application Settings',
    );
    expect(screen.getByTestId('tabs').getAttribute('data-tab-position')).toBe('top');
    expect(screen.getByTestId('tabs').style.width).toBe('100%');
    expect(screen.getByText('base-panel')).toBeTruthy();
    expect(mockTabs).toHaveBeenLastCalledWith(
      expect.objectContaining({ activeKey: 'base', tabPosition: 'top' }),
    );
  });

  it('keeps desktop tabs on the left and writes a selected key to the query string', () => {
    mockSearchKey = 'theme';

    render(<Settings />);

    expect(screen.getByText('theme-panel')).toBeTruthy();
    const tabsProps = mockTabs.mock.calls.at(-1)?.[0];
    expect(tabsProps).toEqual(expect.objectContaining({ activeKey: 'theme', tabPosition: 'left' }));

    tabsProps.onTabClick('security');
    expect(mockSetSearchParams).toHaveBeenCalledWith({ key: 'security' });
  });
});
