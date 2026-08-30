import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { Button, Form } from 'antd';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';
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

  it('preserves the Form.Item label, value, interaction, and submit contract', async () => {
    const onFinish = vi.fn();
    render(
      <Form
        initialValues={{
          code: 'SUP-001',
          email: 'first@example.test',
          creditLevel: 'preferred',
          enabledFilter: 'all',
          enabled: true,
        }}
        onFinish={onFinish}
      >
        <Form.Item label="Code" name="code">
          <PresentationFieldControl {...labels} component="input" />
        </Form.Item>
        <Form.Item label="Contact Email" name="email">
          <PresentationFieldControl {...labels} component="email-input" />
        </Form.Item>
        <Form.Item label="Credit Level" name="creditLevel">
          <PresentationFieldControl
            {...labels}
            component="select"
            options={[
              { value: 'preferred', label: 'Preferred' },
              { value: 'restricted', label: 'Restricted' },
            ]}
          />
        </Form.Item>
        <Form.Item label="Enabled filter" name="enabledFilter">
          <PresentationFieldControl {...labels} component="boolean-filter" />
        </Form.Item>
        <Form.Item label="Enabled" name="enabled" valuePropName="checked">
          <PresentationFieldControl {...labels} component="switch" />
        </Form.Item>
        <Button htmlType="submit">Submit</Button>
      </Form>,
    );

    const code = screen.getByRole('textbox', { name: 'Code' }) as HTMLInputElement;
    const email = screen.getByRole('textbox', { name: 'Contact Email' }) as HTMLInputElement;
    const creditLevel = screen.getByRole('combobox', { name: 'Credit Level' });
    const enabledFilter = screen.getByRole('combobox', { name: 'Enabled filter' });
    const enabled = screen.getByRole('switch', { name: 'Enabled' });

    expect(code.value).toBe('SUP-001');
    expect(email.value).toBe('first@example.test');
    expect(screen.getByText('Preferred')).toBeTruthy();
    expect(screen.getByText('All')).toBeTruthy();
    expect(enabled.getAttribute('aria-checked')).toBe('true');

    fireEvent.change(code, { target: { value: 'SUP-002' } });
    fireEvent.change(email, { target: { value: 'second@example.test' } });
    fireEvent.mouseDown(creditLevel);
    fireEvent.click(await screen.findByRole('option', { name: 'Restricted' }));
    fireEvent.mouseDown(enabledFilter);
    fireEvent.click(await screen.findByRole('option', { name: 'Disabled' }));
    fireEvent.click(enabled);
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }));

    await waitFor(() =>
      expect(onFinish).toHaveBeenCalledWith({
        code: 'SUP-002',
        email: 'second@example.test',
        creditLevel: 'restricted',
        enabledFilter: 'false',
        enabled: false,
      }),
    );
  });
});
