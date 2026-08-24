import { describe, expect, it } from 'vitest';
import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminPagePresentationProfile,
  buildPageRenderModel,
  type PagePresentationSpec,
  resolvePagePresentation,
  validatePageCapabilityDefinition,
  validatePagePresentationProfile,
} from './contract';
import { supplierPresentationCapability } from './supplier.prototype';

const allPermissions = new Set([
  '/suppliers',
  '/suppliers/permissions/list',
  '/suppliers/permissions/read',
  '/suppliers/permissions/create',
  '/suppliers/permissions/update',
  '/suppliers/permissions/delete',
  '/suppliers/permissions/export',
]);

function profile(
  name: string,
  scope: AdminPagePresentationProfile['metadata']['scope'],
  spec: PagePresentationSpec,
): AdminPagePresentationProfile {
  return {
    apiVersion: ADMIN_PRESENTATION_API_VERSION,
    kind: ADMIN_PRESENTATION_KIND,
    metadata: {
      name,
      pageKey: supplierPresentationCapability.pageKey,
      definitionHash: supplierPresentationCapability.definitionHash,
      scope,
    },
    spec,
  };
}

describe('Admin page presentation contract', () => {
  it('accepts the trusted Supplier definition and never mutates it while resolving', () => {
    expect(validatePageCapabilityDefinition(supplierPresentationCapability)).toEqual([]);
    const before = JSON.stringify(supplierPresentationCapability);

    resolvePagePresentation(
      supplierPresentationCapability,
      {
        application: profile(
          'application-layout',
          { kind: 'application' },
          {
            list: { columns: [{ field: 'country', hidden: true }] },
          },
        ),
      },
      allPermissions,
    );

    expect(JSON.stringify(supplierPresentationCapability)).toBe(before);
  });

  it('resolves code, application, role, and user values in deterministic order', () => {
    const application = profile(
      'application-layout',
      { kind: 'application' },
      {
        title: { 'zh-CN': '应用供应商' },
        list: {
          density: 'compact',
          columns: [{ field: 'country', hidden: true }],
        },
      },
    );
    const role = profile(
      'procurement-layout',
      { kind: 'role', subject: 'procurement' },
      {
        title: { 'en-US': 'Procurement suppliers' },
        list: { columns: [{ field: 'country', hidden: false, order: 15 }] },
      },
    );
    const user = profile(
      'operator-layout',
      { kind: 'user', subject: 'operator-1' },
      {
        title: { 'zh-CN': '我的供应商' },
        list: { columns: [{ field: 'country', label: { 'zh-CN': '所在地区' } }] },
      },
    );

    const resolution = resolvePagePresentation(
      supplierPresentationCapability,
      { application, role, user },
      allPermissions,
    );
    const model = buildPageRenderModel(supplierPresentationCapability, resolution, 'zh-CN');

    expect(resolution.appliedLayers).toEqual(['application', 'role', 'user']);
    expect(resolution.rejectedLayers).toEqual([]);
    expect(model.title).toBe('我的供应商');
    expect(model.list.density).toBe('compact');
    expect(model.list.columns[1]).toMatchObject({ field: 'country', label: '所在地区' });
  });

  it('rejects a stale profile as a whole and keeps the compiled fallback', () => {
    const stale = profile(
      'stale-layout',
      { kind: 'application' },
      {
        list: { columns: [{ field: 'country', hidden: true }] },
      },
    );
    stale.metadata.definitionHash = `sha256:${'f'.repeat(64)}`;

    const resolution = resolvePagePresentation(
      supplierPresentationCapability,
      { application: stale },
      allPermissions,
    );
    const model = buildPageRenderModel(supplierPresentationCapability, resolution, 'en-US');

    expect(resolution.appliedLayers).toEqual([]);
    expect(resolution.rejectedLayers[0]?.issues).toContainEqual(
      expect.objectContaining({ code: 'definition-hash-mismatch' }),
    );
    expect(model.list.columns.map((field) => field.field)).toContain('country');
  });

  it('rejects unknown references, forbidden transport keys, and unsafe required-field hiding', () => {
    const invalid = profile(
      'invalid-layout',
      { kind: 'application' },
      {
        list: {
          columns: [{ field: 'invented', component: 'remote-widget' }],
        },
        form: {
          fields: [{ field: 'code', hidden: true }],
        },
      },
    );
    Object.assign(invalid.spec, { url: 'https://example.test/data', method: 'GET' });

    const issues = validatePagePresentationProfile(supplierPresentationCapability, invalid);
    expect(issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: 'unknown-field' }),
        expect.objectContaining({ code: 'required-form-field-hidden' }),
        expect.objectContaining({ code: 'forbidden-profile-key', path: '$.spec.url' }),
        expect.objectContaining({ code: 'forbidden-profile-key', path: '$.spec.method' }),
      ]),
    );

    const resolution = resolvePagePresentation(
      supplierPresentationCapability,
      { application: invalid },
      allPermissions,
    );
    expect(resolution.appliedLayers).toEqual([]);
    expect(resolution.rejectedLayers).toHaveLength(1);
  });

  it('intersects trusted action permissions after every valid presentation overlay', () => {
    const visibleActions = profile(
      'visible-actions',
      { kind: 'application' },
      {
        actions: [
          { action: 'supplier.create', hidden: false },
          { action: 'supplier.delete', hidden: false },
          { action: 'supplier.export', hidden: false },
        ],
      },
    );
    const resolution = resolvePagePresentation(
      supplierPresentationCapability,
      { application: visibleActions },
      new Set(['/suppliers', '/suppliers/permissions/list', '/suppliers/permissions/read']),
    );
    const model = buildPageRenderModel(supplierPresentationCapability, resolution, 'en-US');

    expect(model.status).toBe('ready');
    expect(model.actions.map((action) => action.action)).toEqual(['supplier.read']);
    expect(resolution.removedActionIds).toEqual([
      'supplier.create',
      'supplier.delete',
      'supplier.export',
      'supplier.update',
    ]);
  });

  it('returns a permission-denied model with no data source or actions', () => {
    const resolution = resolvePagePresentation(
      supplierPresentationCapability,
      {},
      new Set(['/suppliers/permissions/create', '/suppliers/permissions/delete']),
    );
    const model = buildPageRenderModel(supplierPresentationCapability, resolution, 'zh-CN');

    expect(model.status).toBe('permission-denied');
    expect(model.dataSource).toBeUndefined();
    expect(model.actions).toEqual([]);
  });

  it('fails closed when trusted capability identifiers are duplicated', () => {
    const firstComponent = supplierPresentationCapability.components[0];
    if (!firstComponent) throw new Error('Supplier needs at least one registered component');
    const invalidCapability = {
      ...supplierPresentationCapability,
      components: [...supplierPresentationCapability.components, firstComponent],
    };

    expect(() => resolvePagePresentation(invalidCapability, {}, allPermissions)).toThrow(
      'Invalid page capability supplier.list',
    );
  });
});
