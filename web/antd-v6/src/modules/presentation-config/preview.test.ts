import { describe, expect, it } from 'vitest';
import { parsePresentationCapabilityCatalog, parsePresentationDocumentText } from './contract';
import { buildPresentationPreview } from './preview';

const definitionHash = `sha256:${'a'.repeat(64)}`;
const capability = parsePresentationCapabilityCatalog({
  recoveryMode: false,
  items: [
    {
      pageKey: 'orders.list',
      definitionVersion: '1',
      definitionHash,
      components: [{ id: 'text' }],
      fields: [
        {
          id: 'status',
          label: { 'zh-CN': '状态', 'en-US': 'Status' },
          valueType: 'enum',
          required: true,
          sortable: true,
          filterable: true,
          surfaces: ['list', 'search', 'form', 'detail'],
          components: ['text'],
        },
      ],
      dataSources: [{ id: 'orders.list', requiredPermissions: ['/orders'] }],
      actions: [{ id: 'orders.read', requiredPermissions: ['/orders/read'], placements: ['row'] }],
      defaultPresentation: {
        title: { 'zh-CN': '订单', 'en-US': 'Orders' },
        dataSource: 'orders.list',
        list: {
          columns: [{ field: 'status', component: 'text', order: 10, hidden: false }],
          density: 'middle',
          pageSize: 20,
          defaultSort: [{ field: 'status', direction: 'asc' }],
        },
        search: {
          fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
          collapsedByDefault: false,
        },
        form: {
          fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
          columns: 1,
        },
        detail: {
          fields: [{ field: 'status', component: 'text', order: 10, hidden: false }],
          columns: 1,
        },
        actions: [{ action: 'orders.read', placement: 'row', order: 10, hidden: false }],
      },
    },
  ],
}).items[0];

describe('presentation preview', () => {
  it('uses only the registered capability plus the validated draft overlay', () => {
    if (!capability) throw new Error('capability fixture is missing');
    const document = parsePresentationDocumentText(
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
          title: { 'en-US': 'Priority orders' },
          list: { density: 'compact', pageSize: 50 },
        },
      }),
    );
    const preview = buildPresentationPreview(capability, document, 'en-US');

    expect(preview).toMatchObject({
      pageKey: 'orders.list',
      status: 'ready',
      title: 'Priority orders',
      dataSource: 'orders.list',
      list: { density: 'compact', pageSize: 50 },
    });
    expect(preview.actions.map((action) => action.action)).toEqual(['orders.read']);
  });
});
