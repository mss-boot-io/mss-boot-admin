import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { usePresentationIntl } from './messages';

const runtime = vi.hoisted(() => ({ formatMessage: vi.fn(), locale: 'zh-CN' }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ locale: runtime.locale, messages: {}, formatMessage: runtime.formatMessage }),
}));

function Harness() {
  const intl = usePresentationIntl();
  return (
    <>
      <span>{intl.formatMessage({ id: 'presentation.title' })}</span>
      <span>{intl.formatMessage({ id: 'presentation.adoption.active.title' })}</span>
      <span>{intl.formatMessage({ id: 'presentation.adoption.activePages' })}</span>
      <span>{intl.formatMessage({ id: 'presentation.recovery.description' })}</span>
      <span>{intl.formatMessage({ id: 'presentation.conflict.description' }, { version: 7 })}</span>
    </>
  );
}

describe('presentation messages', () => {
  beforeEach(() => {
    runtime.locale = 'zh-CN';
    runtime.formatMessage.mockReset();
    runtime.formatMessage.mockImplementation(
      ({ defaultMessage }: { defaultMessage?: string }, values?: Record<string, unknown>) =>
        (defaultMessage ?? '').replace('{version}', String(values?.version ?? '')),
    );
  });

  it('formats bundled messages without asking React Intl for an unregistered id', () => {
    render(<Harness />);

    expect(screen.getByText('页面展示')).toBeTruthy();
    expect(screen.getByText('运行时展示配置已启用')).toBeTruthy();
    expect(screen.getByText('已配置活动页面：')).toBeTruthy();
    expect(
      screen.getByText(
        '恢复模式会覆盖运行时采用策略；页面使用代码默认值，同时仍可管理配置档案和发布历史。',
      ),
    ).toBeTruthy();
    expect(screen.getByText('本地 JSON 已保留；服务端版本为 7。')).toBeTruthy();
    expect(runtime.formatMessage).not.toHaveBeenCalled();
  });

  it('keeps the English governance copy synchronized', () => {
    runtime.locale = 'en-US';
    render(<Harness />);

    expect(screen.getByText('Presentation')).toBeTruthy();
    expect(screen.getByText('Runtime presentation adoption is active')).toBeTruthy();
    expect(screen.getByText('Configured active pages:')).toBeTruthy();
    expect(
      screen.getByText(
        'Recovery overrides runtime adoption. Pages use compiled defaults while profiles and publication history remain manageable.',
      ),
    ).toBeTruthy();
    expect(screen.getByText('Local JSON is preserved. Server version: 7.')).toBeTruthy();
    expect(runtime.formatMessage).not.toHaveBeenCalled();
  });
});
