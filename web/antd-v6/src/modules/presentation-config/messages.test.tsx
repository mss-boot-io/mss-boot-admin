import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { usePresentationIntl } from './messages';

const runtime = vi.hoisted(() => ({ formatMessage: vi.fn() }));

vi.mock('@umijs/max', () => ({
  useIntl: () => ({ locale: 'zh-CN', messages: {}, formatMessage: runtime.formatMessage }),
}));

function Harness() {
  const intl = usePresentationIntl();
  return (
    <>
      <span>{intl.formatMessage({ id: 'presentation.title' })}</span>
      <span>{intl.formatMessage({ id: 'presentation.conflict.description' }, { version: 7 })}</span>
    </>
  );
}

describe('presentation messages', () => {
  beforeEach(() => {
    runtime.formatMessage.mockReset();
    runtime.formatMessage.mockImplementation(
      ({ defaultMessage }: { defaultMessage?: string }, values?: Record<string, unknown>) =>
        (defaultMessage ?? '').replace('{version}', String(values?.version ?? '')),
    );
  });

  it('formats bundled messages without asking React Intl for an unregistered id', () => {
    render(<Harness />);

    expect(screen.getByText('页面展示')).toBeTruthy();
    expect(screen.getByText('本地 JSON 已保留；服务端版本为 7。')).toBeTruthy();
    expect(runtime.formatMessage).not.toHaveBeenCalled();
  });
});
