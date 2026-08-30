import { corePresentationRegistry } from '@mss-admin-core/generated/core-presentation-registry.generated';
import { supplierPresentationDefinition } from '@mss-admin-core/generated/modules/supplier/presentation.generated';
import type { PageCapabilityDefinition } from '@mss-admin-core/shared/presentation/contract';
import { fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import type { PresentationCapability, PresentationJSONObject } from './contract';
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

function editorCapability(definition: PageCapabilityDefinition): PresentationCapability {
  return {
    pageKey: definition.pageKey,
    definitionVersion: definition.definitionVersion,
    definitionHash: definition.definitionHash,
    components: definition.components.map((component) => component.id),
    fields: definition.fields.map((field) => ({
      id: field.id,
      label: { ...field.label },
      valueType: field.valueType,
      required: field.required,
      sortable: field.sortable,
      filterable: field.filterable,
      surfaces: [...field.surfaces],
      components: [...field.components],
    })),
    dataSources: definition.dataSources.map((dataSource) => dataSource.id),
    actions: definition.actions.map((action) => action.id),
    defaultPresentation: definition.defaultPresentation as unknown as PresentationJSONObject,
    definition,
  };
}

const limitedDefinition = corePresentationRegistry['user.list']
  .definition as unknown as PageCapabilityDefinition;
const limitedCapability = editorCapability(limitedDefinition);
const limitedDocument = parsePresentationDraftAST(
  JSON.stringify({
    apiVersion: 'mss.io/v1alpha1',
    kind: 'AdminPagePresentation',
    metadata: {
      name: 'user-application',
      pageKey: limitedDefinition.pageKey,
      definitionHash: limitedDefinition.definitionHash,
      scope: { kind: 'application' },
    },
    spec: {},
  }),
);
const supplierDefinition = supplierPresentationDefinition as PageCapabilityDefinition;
const supplierCapability = editorCapability(supplierDefinition);
const supplierDocument = parsePresentationDraftAST(
  JSON.stringify({
    apiVersion: 'mss.io/v1alpha1',
    kind: 'AdminPagePresentation',
    metadata: {
      name: 'supplier-application',
      pageKey: supplierDefinition.pageKey,
      definitionHash: supplierDefinition.definitionHash,
      scope: { kind: 'application' },
    },
    spec: {},
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

function LimitedHarness() {
  const [document, setDocument] = useState<PresentationDraftAST>(limitedDocument);
  return (
    <PresentationVisualEditor
      capability={limitedCapability}
      document={document}
      onChange={setDocument}
    />
  );
}

function SupplierHarness() {
  const [document, setDocument] = useState<PresentationDraftAST>(supplierDocument);
  return (
    <PresentationVisualEditor
      capability={supplierCapability}
      document={document}
      onChange={setDocument}
    />
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

  it('only exposes general, list, and search tabs for limited core table capabilities', () => {
    render(<LimitedHarness />);
    expect(screen.getByText('presentation.visual.general')).toBeTruthy();
    expect(screen.getByText('presentation.visual.list')).toBeTruthy();
    expect(screen.getByText('presentation.visual.search')).toBeTruthy();
    expect(screen.queryByText('presentation.visual.form')).toBeNull();
    expect(screen.queryByText('presentation.visual.detail')).toBeNull();
    expect(screen.queryByText('presentation.visual.actions')).toBeNull();
    expect(screen.queryByText('presentation.visual.dataSource')).toBeNull();

    fireEvent.click(screen.getByText('presentation.visual.list'));
    expect(screen.queryByText('presentation.visual.defaultSort')).toBeNull();
    expect(screen.queryByLabelText('email presentation.visual.component')).toBeNull();

    fireEvent.click(screen.getByText('presentation.visual.search'));

    expect(screen.queryByLabelText('name presentation.visual.component')).toBeNull();
    expect(screen.queryByText('presentation.visual.placeholder')).toBeNull();
    expect(screen.queryByText('presentation.visual.help')).toBeNull();
    expect(
      screen.getAllByText('presentation.visual.condition.limited.disabled').length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText('presentation.visual.condition.add')).toBeNull();
  });

  it('keeps every presentation tab for the external Supplier extension capability', () => {
    render(<SupplierHarness />);

    for (const tab of ['general', 'list', 'search', 'form', 'detail', 'actions']) {
      expect(screen.getByText(`presentation.visual.${tab}`)).toBeTruthy();
    }

    expect(screen.getByText('presentation.visual.dataSource')).toBeTruthy();
    fireEvent.click(screen.getByText('presentation.visual.list'));
    expect(screen.getByText('presentation.visual.defaultSort')).toBeTruthy();
    expect(screen.getByLabelText('code presentation.visual.component')).toBeTruthy();

    fireEvent.click(screen.getByText('presentation.visual.search'));
    expect(screen.getAllByLabelText('code presentation.visual.component').length).toBeGreaterThan(
      1,
    );
    expect(screen.getAllByText('presentation.visual.placeholder').length).toBeGreaterThan(0);
    expect(screen.getAllByText('presentation.visual.help').length).toBeGreaterThan(0);
  });
});
