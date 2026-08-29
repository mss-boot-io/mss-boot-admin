import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { PresentationFieldControl, renderPresentationValue } from './components';

const labels = {
  allLabel: 'All',
  enabledLabel: 'Enabled',
  disabledLabel: 'Disabled',
};

describe('compiled presentation component registry', () => {
  it('renders every trusted read component without dynamic lookup', () => {
    const render = (component: string, value: unknown) =>
      renderToStaticMarkup(
        renderPresentationValue({
          component,
          value,
          options: [{ value: 'preferred', label: 'Preferred', color: 'green' }],
          enabledLabel: 'Enabled',
          disabledLabel: 'Disabled',
          formatDate: () => 'Formatted date',
        }),
      );

    expect(render('text', 'Supplier')).toContain('Supplier');
    expect(render('tag', 'preferred')).toContain('Preferred');
    expect(render('boolean', true)).toContain('Enabled');
    expect(render('date-time', '2026-01-01T00:00:00Z')).toContain('Formatted date');
    expect(render('copyable-code', 'SUP-1')).toContain('SUP-1');
    expect(render('unregistered', 'unsafe')).toBe('');
  });

  it('renders every trusted editable component and rejects unknown IDs', () => {
    const render = (component: string) =>
      renderToStaticMarkup(
        <PresentationFieldControl
          {...labels}
          component={component}
          options={[{ value: 'preferred', label: 'Preferred' }]}
          placeholder="Configured placeholder"
        />,
      );

    expect(render('input')).toContain('Configured placeholder');
    expect(render('email-input')).toContain('type="email"');
    expect(render('select')).toContain('ant-select');
    expect(render('boolean-filter')).toContain('ant-select');
    expect(render('switch')).toContain('ant-switch');
    expect(render('unregistered')).toBe('');
  });
});
