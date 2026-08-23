import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminLocale,
  type AdminPagePresentationProfile,
  buildPageRenderModel,
  type PageCapabilityDefinition,
  type PageRenderModel,
  resolvePagePresentation,
} from './contract';

/**
 * P0 reference only. This compatibility surface mirrors the existing Supplier
 * AdminModule but is deliberately not imported by the production route tree.
 * A production adoption must move this projection into the deterministic
 * AdminModule generator instead of hand-maintaining another source of truth.
 */
export const supplierPresentationCompatibility = {
  pageKey: 'supplier.list',
  definitionVersion: '1',
  components: [
    { id: 'boolean' },
    { id: 'email-input' },
    { id: 'input' },
    { id: 'select' },
    { id: 'switch' },
    { id: 'tag' },
    { id: 'text' },
  ],
  fields: [
    {
      id: 'code',
      label: { 'zh-CN': '供应商编码', 'en-US': 'Code' },
      valueType: 'string',
      required: true,
      sortable: true,
      filterable: false,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['text', 'input'],
    },
    {
      id: 'name',
      label: { 'zh-CN': '供应商名称', 'en-US': 'Name' },
      valueType: 'string',
      required: true,
      sortable: true,
      filterable: false,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['text', 'input'],
    },
    {
      id: 'country',
      label: { 'zh-CN': '国家或地区', 'en-US': 'Country or region' },
      valueType: 'string',
      required: false,
      sortable: false,
      filterable: true,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['text', 'input'],
    },
    {
      id: 'contactName',
      label: { 'zh-CN': '联系人', 'en-US': 'Contact' },
      valueType: 'string',
      required: true,
      sortable: false,
      filterable: false,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['text', 'input'],
    },
    {
      id: 'contactEmail',
      label: { 'zh-CN': '联系邮箱', 'en-US': 'Contact email' },
      valueType: 'string',
      required: false,
      sortable: false,
      filterable: false,
      surfaces: ['list', 'form', 'detail'],
      components: ['text', 'email-input'],
    },
    {
      id: 'creditLevel',
      label: { 'zh-CN': '信用等级', 'en-US': 'Credit level' },
      valueType: 'enum',
      required: true,
      sortable: false,
      filterable: true,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['tag', 'select'],
    },
    {
      id: 'enabled',
      label: { 'zh-CN': '启用状态', 'en-US': 'Enabled' },
      valueType: 'boolean',
      required: true,
      sortable: false,
      filterable: true,
      surfaces: ['list', 'search', 'form', 'detail'],
      components: ['boolean', 'switch'],
    },
  ],
  dataSources: [
    {
      id: 'supplier.list',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/list'],
    },
  ],
  actions: [
    {
      id: 'supplier.create',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/create'],
      placements: ['toolbar'],
    },
    {
      id: 'supplier.read',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/read'],
      placements: ['row', 'detail'],
    },
    {
      id: 'supplier.update',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/update'],
      placements: ['row', 'form'],
    },
    {
      id: 'supplier.delete',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/delete'],
      placements: ['row'],
      destructive: true,
    },
    {
      id: 'supplier.export',
      requiredPermissions: ['/suppliers', '/suppliers/permissions/export'],
      placements: ['toolbar'],
    },
  ],
} as const;

export const supplierPresentationCapability: PageCapabilityDefinition = {
  ...supplierPresentationCompatibility,
  definitionHash: 'sha256:276e11ca561729c8b288c4bb045364dbc442ba56ccc5fb59ffdab19f7e5ee473',
  defaultPresentation: {
    title: { 'zh-CN': '供应商管理', 'en-US': 'Suppliers' },
    dataSource: 'supplier.list',
    list: {
      density: 'middle',
      pageSize: 20,
      defaultSort: [{ field: 'code', direction: 'asc' }],
      columns: [
        { field: 'code', component: 'text', order: 10, hidden: false, width: 180 },
        { field: 'name', component: 'text', order: 20, hidden: false, width: 240 },
        { field: 'country', component: 'text', order: 30, hidden: false },
        { field: 'contactName', component: 'text', order: 40, hidden: false },
        { field: 'contactEmail', component: 'text', order: 50, hidden: false },
        { field: 'creditLevel', component: 'tag', order: 60, hidden: false },
        { field: 'enabled', component: 'boolean', order: 70, hidden: false },
      ],
    },
    search: {
      collapsedByDefault: true,
      fields: [
        { field: 'code', component: 'input', order: 10, hidden: false },
        { field: 'name', component: 'input', order: 20, hidden: false },
        { field: 'country', component: 'input', order: 30, hidden: false },
        { field: 'contactName', component: 'input', order: 40, hidden: false },
        { field: 'creditLevel', component: 'select', order: 50, hidden: false },
        { field: 'enabled', component: 'switch', order: 60, hidden: false },
      ],
    },
    form: {
      columns: 2,
      fields: [
        { field: 'code', component: 'input', order: 10, hidden: false, span: 12 },
        { field: 'name', component: 'input', order: 20, hidden: false, span: 12 },
        { field: 'country', component: 'input', order: 30, hidden: false, span: 12 },
        { field: 'contactName', component: 'input', order: 40, hidden: false, span: 12 },
        {
          field: 'contactEmail',
          component: 'email-input',
          order: 50,
          hidden: false,
          span: 12,
        },
        { field: 'creditLevel', component: 'select', order: 60, hidden: false, span: 12 },
        { field: 'enabled', component: 'switch', order: 70, hidden: false, span: 12 },
      ],
    },
    detail: {
      columns: 2,
      fields: [
        { field: 'code', component: 'text', order: 10, hidden: false, span: 12 },
        { field: 'name', component: 'text', order: 20, hidden: false, span: 12 },
        { field: 'country', component: 'text', order: 30, hidden: false, span: 12 },
        { field: 'contactName', component: 'text', order: 40, hidden: false, span: 12 },
        { field: 'contactEmail', component: 'text', order: 50, hidden: false, span: 12 },
        { field: 'creditLevel', component: 'tag', order: 60, hidden: false, span: 12 },
        { field: 'enabled', component: 'boolean', order: 70, hidden: false, span: 12 },
      ],
    },
    actions: [
      {
        action: 'supplier.create',
        label: { 'zh-CN': '新建供应商', 'en-US': 'Create supplier' },
        placement: 'toolbar',
        order: 10,
        hidden: false,
      },
      {
        action: 'supplier.export',
        label: { 'zh-CN': '导出', 'en-US': 'Export' },
        placement: 'toolbar',
        order: 20,
        hidden: false,
      },
      {
        action: 'supplier.read',
        label: { 'zh-CN': '查看', 'en-US': 'View' },
        placement: 'row',
        order: 30,
        hidden: false,
      },
      {
        action: 'supplier.update',
        label: { 'zh-CN': '编辑', 'en-US': 'Edit' },
        placement: 'row',
        order: 40,
        hidden: false,
      },
      {
        action: 'supplier.delete',
        label: { 'zh-CN': '删除', 'en-US': 'Delete' },
        placement: 'row',
        order: 50,
        hidden: false,
        confirm: {
          'zh-CN': '确认删除该供应商？',
          'en-US': 'Delete this supplier?',
        },
      },
    ],
  },
};

export const supplierCompactApplicationProfile: AdminPagePresentationProfile = {
  apiVersion: ADMIN_PRESENTATION_API_VERSION,
  kind: ADMIN_PRESENTATION_KIND,
  metadata: {
    name: 'supplier-compact',
    pageKey: supplierPresentationCapability.pageKey,
    description: 'P0 compact Supplier list reference without persistence or route registration.',
    definitionHash: supplierPresentationCapability.definitionHash,
    scope: { kind: 'application' },
  },
  spec: {
    title: { 'zh-CN': '供应商概览', 'en-US': 'Supplier overview' },
    list: {
      density: 'compact',
      pageSize: 50,
      columns: [
        { field: 'country', hidden: true },
        { field: 'contactName', hidden: true },
        { field: 'contactEmail', hidden: true },
        { field: 'enabled', order: 25, label: { 'zh-CN': '状态', 'en-US': 'Status' } },
      ],
    },
    search: {
      collapsedByDefault: false,
      fields: [{ field: 'contactName', hidden: true }],
    },
    actions: [
      { action: 'supplier.export', hidden: true },
      { action: 'supplier.delete', hidden: true },
    ],
  },
};

export function buildSupplierCompactPrototype(
  grantedPermissions: ReadonlySet<string>,
  locale: AdminLocale = 'zh-CN',
): PageRenderModel {
  const resolution = resolvePagePresentation(
    supplierPresentationCapability,
    { application: supplierCompactApplicationProfile },
    grantedPermissions,
  );
  return buildPageRenderModel(supplierPresentationCapability, resolution, locale);
}
