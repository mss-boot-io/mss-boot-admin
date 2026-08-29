import { fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { PresentationCapability } from './contract';
import PresentationVisualEditor from './PresentationVisualEditor';
import { type PresentationDraftAST, parsePresentationDraftAST } from './presentationDraftAst';

vi.mock('@umijs/max', () => ({
  useIntl: () => ({
    locale: 'en-US',
    formatMessage: ({ id }: { id: string }) => id,
  }),
}));

const definitionHash = `sha256:${'a'.repeat(64)}`;
const fieldBase = {
  label: { 'en-US': 'Status', 'zh-CN': '状态' },
  valueType: 'enum' as const,
  format: 'plain' as const,
  required: true,
  nullable: false,
  readOnly: false,
  searchable: true,
  sortable: true,
  filterable: true,
  surfaces: ['list', 'search', 'form', 'detail'] as const,
  components: ['input', 'select', 'tag', 'text'],
  surfaceComponents: [
    { surface: 'list' as const, components: ['text'] },
    { surface: 'search' as const, components: ['select'] },
    { surface: 'form' as const, components: ['select'] },
    { surface: 'detail' as const, components: ['tag'] },
  ],
};
const capability = {
  pageKey: 'orders.list',
  definitionVersion: '2',
  definitionHash,
  components: ['input', 'select', 'tag', 'text'],
  fields: [
    {
      id: 'status',
      label: fieldBase.label,
      valueType: 'enum',
      required: true,
      sortable: true,
      filterable: true,
      surfaces: [...fieldBase.surfaces],
      components: [...fieldBase.components],
    },
  ],
  dataSources: ['orders.list'],
  actions: ['orders.export', 'orders.read'],
  defaultPresentation: { title: { 'en-US': 'Orders' } },
  definition: {
    pageKey: 'orders.list',
    definitionVersion: '2',
    definitionHash,
    components: ['input', 'select', 'tag', 'text'].map((id) => ({ id })),
    fields: [{ id: 'status', ...fieldBase }],
    dataSources: [
      {
        id: 'orders.list',
        requiredPermissions: ['/orders'],
        pageSizeOptions: [20, 50],
        maxPageSize: 50,
        maxSortFields: 1,
      },
    ],
    actions: [
      { id: 'orders.export', requiredPermissions: ['/orders/export'], placements: ['toolbar'] },
      { id: 'orders.read', requiredPermissions: ['/orders/read'], placements: ['row', 'detail'] },
    ],
    defaultPresentation: {
      title: { 'en-US': 'Orders' },
      dataSource: 'orders.list',
      list: {
        columns: [{ field: 'status', component: 'text', order: 10, hidden: false, width: 160 }],
        density: 'middle',
        pageSize: 20,
        defaultSort: [],
      },
      search: {
        fields: [{ field: 'status', component: 'select', order: 10, hidden: false }],
        collapsedByDefault: false,
      },
      form: {
        fields: [{ field: 'status', component: 'select', order: 10, hidden: false, span: 24 }],
        columns: 1,
      },
      detail: {
        fields: [{ field: 'status', component: 'tag', order: 10, hidden: false, span: 24 }],
        columns: 1,
      },
      actions: [
        { action: 'orders.export', placement: 'toolbar', order: 10, hidden: false },
        { action: 'orders.read', placement: 'row', order: 20, hidden: false },
      ],
    },
  },
} satisfies PresentationCapability;

const initialDocument = parsePresentationDraftAST(
  JSON.stringify({
    apiVersion: 'mss.io/v1alpha1',
    kind: 'AdminPagePresentation',
    metadata: {
      name: 'orders-application',
      pageKey: 'orders.list',
      definitionHash,
      scope: { kind: 'application' },
    },
    spec: {
      search: { collapsedByDefault: false },
      detail: {
        fields: [
          {
            field: 'status',
            hidden: false,
            visibleWhen: {
              all: [
                { field: 'status', operator: 'exists' },
                { not: { field: 'status', operator: 'eq', value: 'closed' } },
              ],
            },
          },
        ],
      },
      actions: [{ action: 'orders.read', placement: 'row', hidden: false }],
    },
    futureRoot: { enabled: false },
  }),
);

function Harness() {
  const [document, setDocument] = useState<PresentationDraftAST>(initialDocument);
  return (
    <>
      <PresentationVisualEditor
        capability={capability}
        document={document}
        onChange={setDocument}
      />
      <output data-testid="document">{JSON.stringify(document)}</output>
    </>
  );
}

describe('presentation visual editor', () => {
  it('edits through the shared AST without losing false, conditions, or unknown nodes', () => {
    render(<Harness />);
    fireEvent.change(screen.getByLabelText('presentation.visual.title en-US'), {
      target: { value: 'Configured orders' },
    });

    const document = JSON.parse(screen.getByTestId('document').textContent ?? '{}');
    expect(document.spec.title['en-US']).toBe('Configured orders');
    expect(document.spec.search.collapsedByDefault).toBe(false);
    expect(document.spec.detail.fields[0].hidden).toBe(false);
    expect(document.spec.detail.fields[0].visibleWhen.all).toHaveLength(2);
    expect(document.futureRoot.enabled).toBe(false);
  });

  it('offers only surface-compatible components and protects required form visibility', async () => {
    render(<Harness />);
    fireEvent.click(screen.getByText('presentation.visual.list'));
    fireEvent.mouseDown(screen.getByLabelText('status presentation.visual.component'));
    expect((await screen.findAllByText('text')).length).toBeGreaterThan(0);
    expect(screen.queryByText('input')).toBeNull();
    expect(screen.queryByText('presentation.visual.condition.add')).toBeNull();

    fireEvent.click(screen.getByText('presentation.visual.form'));
    expect(screen.getByText('presentation.visual.condition.required.disabled')).toBeTruthy();
    const formHidden = screen.getAllByLabelText('status presentation.visual.hidden').at(-1);
    if (!formHidden) throw new Error('required form visibility control is missing');
    fireEvent.mouseDown(formHidden);
    const hiddenTrue = await screen.findByRole('option', {
      name: 'presentation.visual.true',
    });
    expect(hiddenTrue.getAttribute('aria-disabled')).toBe('true');
  });

  it('preserves a compound condition and exposes bounded action conditions only off toolbar', () => {
    render(<Harness />);
    fireEvent.click(screen.getByText('presentation.visual.detail'));
    expect(screen.getByText('presentation.visual.condition.compound')).toBeTruthy();

    fireEvent.click(screen.getByText('presentation.visual.actions'));
    expect(screen.getByText('presentation.visual.condition.toolbar.disabled')).toBeTruthy();
    expect(screen.getByText('presentation.visual.condition.add')).toBeTruthy();
  });
});
