import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';
import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminPagePresentationProfile,
  buildPageRenderModel,
  canonicalizeCapabilityContract,
  compilePortablePresentationPattern,
  isLimitedTablePresentationCapability,
  type PageCapabilityDefinition,
  type PagePresentationSpec,
  resolvePagePresentation,
  validatePageCapabilityDefinition,
  validatePagePresentationProfile,
} from './contract';
import { supplierPresentationCapability } from './supplier.prototype';

const canonicalGolden = JSON.parse(
  readFileSync(
    resolve(process.cwd(), '../../internal/mss/spec/testdata/presentation-canonical-v2.json'),
    'utf8',
  ),
) as { value: unknown; canonical: string; sha256: string };

const patternGolden = JSON.parse(
  readFileSync(
    resolve(
      process.cwd(),
      '../../internal/mss/spec/testdata/presentation-pattern-semantics-v2.json',
    ),
    'utf8',
  ),
) as {
  accepted: { pattern: string; matches: string[]; rejects: string[] }[];
  rejected: string[];
};

const allPermissions = new Set([
  '/suppliers',
  '/suppliers/permissions/list',
  '/suppliers/permissions/read',
  '/suppliers/permissions/create',
  '/suppliers/permissions/update',
  '/suppliers/permissions/delete',
  '/suppliers/permissions/export',
]);

const limitedUserDefinition = corePresentationRegistry['user.list']
  .definition as unknown as PageCapabilityDefinition;

function validV2Capability(): PageCapabilityDefinition {
  return {
    ...supplierPresentationCapability,
    definitionVersion: '2',
    fields: supplierPresentationCapability.fields.map((field) =>
      field.valueType === 'enum'
        ? {
            ...field,
            format: 'plain',
            nullable: !field.required,
            readOnly: false,
            searchable: false,
            validation: field.validation ?? {},
            surfaceComponents: field.surfaces.map((surface) => ({
              surface,
              components: field.components,
            })),
            enumValues: [
              { value: 'a', label: { 'zh-CN': '甲级', 'en-US': 'Level A' }, color: '' },
              { value: 'b', label: { 'zh-CN': '乙级', 'en-US': 'Level B' }, color: '' },
            ],
          }
        : {
            ...field,
            format: field.id === 'contactEmail' ? 'email' : 'plain',
            nullable: !field.required,
            readOnly: false,
            searchable: false,
            validation: field.validation ?? {},
            enumValues: [],
            surfaceComponents: field.surfaces.map((surface) => ({
              surface,
              components: field.components,
            })),
          },
    ),
    dataSources: supplierPresentationCapability.dataSources.map((dataSource) => ({
      ...dataSource,
      pageSizeOptions: [20, 50, 100],
      maxPageSize: 100,
      maxSortFields: 1,
    })),
    actions: supplierPresentationCapability.actions.map((action) => ({
      ...action,
      destructive: action.destructive ?? false,
    })),
  };
}

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
  it('matches the shared Go and TypeScript canonical bytes and hash golden', () => {
    const canonical = canonicalizeCapabilityContract(canonicalGolden.value);
    expect(canonical).toBe(canonicalGolden.canonical);
    expect(createHash('sha256').update(canonical).digest('hex')).toBe(canonicalGolden.sha256);
  });

  it('uses the shared Unicode-safe portable pattern semantics', () => {
    for (const test of patternGolden.accepted) {
      const compiled = compilePortablePresentationPattern(test.pattern);
      for (const value of test.matches) expect(compiled.test(value)).toBe(true);
      for (const value of test.rejects) expect(compiled.test(value)).toBe(false);
    }
    for (const pattern of patternGolden.rejected) {
      expect(() => compilePortablePresentationPattern(pattern)).toThrow(/portable/i);
    }
  });

  it('fails closed for an unknown capability definition version', () => {
    const issues = validatePageCapabilityDefinition({
      ...supplierPresentationCapability,
      definitionVersion: '3',
    });

    expect(issues).toContainEqual(
      expect.objectContaining({
        code: 'unsupported-definition-version',
        path: 'definitionVersion',
      }),
    );
  });

  it('rejects every protected namespace and overlong page keys', () => {
    for (const pageKey of [
      'account.users',
      'app-config.theme',
      'authorization.roles',
      'config.theme',
      'login.page',
      'presentation.governance',
      'system.settings',
    ]) {
      expect(
        validatePageCapabilityDefinition({ ...supplierPresentationCapability, pageKey }),
      ).toEqual(expect.arrayContaining([expect.objectContaining({ code: 'protected-page' })]));
    }
    expect(
      validatePageCapabilityDefinition({
        ...supplierPresentationCapability,
        pageKey: `a.${'b'.repeat(119)}`,
      }),
    ).toEqual(expect.arrayContaining([expect.objectContaining({ code: 'invalid-page-key' })]));
  });

  it('keeps the version 1 Supplier definition compatible with optional version 2 facts', () => {
    const extendedV1Capability: PageCapabilityDefinition = {
      ...supplierPresentationCapability,
      fields: supplierPresentationCapability.fields.map((field) => {
        if (field.valueType === 'enum') {
          return {
            ...field,
            enumValues: [{ value: 'a', label: { 'zh-CN': '甲级', 'en-US': 'Level A' } }],
          };
        }
        if (field.id === 'contactEmail') {
          return {
            ...field,
            format: 'email',
            nullable: true,
            readOnly: false,
          };
        }
        return field;
      }),
      dataSources: supplierPresentationCapability.dataSources.map((dataSource) => ({
        ...dataSource,
        pageSizeOptions: [20, 50],
        maxPageSize: 50,
        maxSortFields: 1,
      })),
    };

    expect(validatePageCapabilityDefinition(supplierPresentationCapability)).toEqual([]);
    expect(validatePageCapabilityDefinition(extendedV1Capability)).toEqual([]);
    expect(() => resolvePagePresentation(extendedV1Capability, {}, allPermissions)).not.toThrow();
  });

  it('accepts the historical version 1 definition after Go wire serialization adds zero facts', () => {
    const goWireCapability = JSON.parse(
      JSON.stringify({
        ...supplierPresentationCapability,
        fields: supplierPresentationCapability.fields.map((field) => ({
          ...field,
          format: '',
          nullable: false,
          readOnly: false,
          searchable: false,
          surfaceComponents: null,
          enumValues: null,
          validation: {},
        })),
        actions: supplierPresentationCapability.actions.map((action) => ({
          ...action,
          destructive: action.destructive ?? false,
        })),
      }),
    ) as PageCapabilityDefinition;

    expect(validatePageCapabilityDefinition(goWireCapability)).toEqual([]);
  });

  it('keeps historical qualified version 1 field identifiers compatible', () => {
    const legacyID = 'code-value';
    const rename = (field: string) => (field === 'code' ? legacyID : field);
    const capability: PageCapabilityDefinition = {
      ...supplierPresentationCapability,
      fields: supplierPresentationCapability.fields.map((field) =>
        field.id === 'code' ? { ...field, id: legacyID } : field,
      ),
      defaultPresentation: {
        ...supplierPresentationCapability.defaultPresentation,
        list: {
          ...supplierPresentationCapability.defaultPresentation.list,
          columns: supplierPresentationCapability.defaultPresentation.list.columns.map((field) => ({
            ...field,
            field: rename(field.field),
          })),
          defaultSort: supplierPresentationCapability.defaultPresentation.list.defaultSort.map(
            (sort) => ({ ...sort, field: rename(sort.field) }),
          ),
        },
        search: {
          ...supplierPresentationCapability.defaultPresentation.search,
          fields: supplierPresentationCapability.defaultPresentation.search.fields.map((field) => ({
            ...field,
            field: rename(field.field),
          })),
        },
        form: {
          ...supplierPresentationCapability.defaultPresentation.form,
          fields: supplierPresentationCapability.defaultPresentation.form.fields.map((field) => ({
            ...field,
            field: rename(field.field),
          })),
        },
        detail: {
          ...supplierPresentationCapability.defaultPresentation.detail,
          fields: supplierPresentationCapability.defaultPresentation.detail.fields.map((field) => ({
            ...field,
            field: rename(field.field),
          })),
        },
      },
    };

    expect(validatePageCapabilityDefinition(capability)).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ code: 'invalid-field-identifier' })]),
    );
  });

  it('rejects visibility conditions on limited core table capabilities and keeps compiled runtime', () => {
    const conditionalProfile: AdminPagePresentationProfile = {
      apiVersion: ADMIN_PRESENTATION_API_VERSION,
      kind: ADMIN_PRESENTATION_KIND,
      metadata: {
        name: 'user-application',
        pageKey: limitedUserDefinition.pageKey,
        definitionHash: limitedUserDefinition.definitionHash,
        scope: { kind: 'application' },
      },
      spec: {
        search: {
          fields: [
            {
              field: 'name',
              visibleWhen: { field: 'status', operator: 'eq', value: 'enabled' },
            },
          ],
        },
      },
    };

    expect(isLimitedTablePresentationCapability(limitedUserDefinition)).toBe(true);
    expect(validatePagePresentationProfile(limitedUserDefinition, conditionalProfile)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: 'unsupported-limited-condition',
          path: 'spec.search.fields[0].visibleWhen',
        }),
      ]),
    );

    const resolution = resolvePagePresentation(
      limitedUserDefinition,
      { application: conditionalProfile },
      new Set(['/users']),
    );
    expect(resolution.appliedLayers).toEqual([]);
    expect(resolution.rejectedLayers).toEqual([
      expect.objectContaining({ layer: 'application', profileName: 'user-application' }),
    ]);
    expect(resolution.presentation.search.fields).toEqual(
      limitedUserDefinition.defaultPresentation.search.fields,
    );
  });

  it('requires bounded page-size and sort metadata for every version 2 data source', () => {
    const capability = validV2Capability();
    capability.dataSources = capability.dataSources.map((dataSource) => ({
      id: dataSource.id,
      requiredPermissions: dataSource.requiredPermissions,
    }));

    expect(validatePageCapabilityDefinition(capability)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: 'invalid-max-page-size' }),
        expect.objectContaining({ code: 'missing-page-size-options' }),
        expect.objectContaining({ code: 'invalid-max-sort-fields' }),
      ]),
    );

    const zeroSortLimit = validV2Capability();
    zeroSortLimit.dataSources = zeroSortLimit.dataSources.map((dataSource) => ({
      ...dataSource,
      maxSortFields: 0,
    }));
    expect(validatePageCapabilityDefinition(zeroSortLimit)).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ code: 'invalid-max-sort-fields' })]),
    );
  });

  it('requires every version 2 canonical wire fact to be explicit', () => {
    const missingValidation = validV2Capability();
    missingValidation.fields = missingValidation.fields.map((field, index) =>
      index === 0 ? { ...field, validation: undefined } : field,
    );

    const missingEnumValues = validV2Capability();
    missingEnumValues.fields = missingEnumValues.fields.map((field, index) =>
      index === 0 ? { ...field, enumValues: undefined } : field,
    );

    const missingEnumColor = validV2Capability();
    missingEnumColor.fields = missingEnumColor.fields.map((field) =>
      field.valueType === 'enum'
        ? {
            ...field,
            enumValues: field.enumValues?.map((value, index) =>
              index === 0 ? { ...value, color: undefined } : value,
            ),
          }
        : field,
    );

    const missingActionDestructive = validV2Capability();
    missingActionDestructive.actions = missingActionDestructive.actions.map((action, index) =>
      index === 0 ? { ...action, destructive: undefined } : action,
    );

    for (const [capability, code] of [
      [missingValidation, 'missing-field-validation'],
      [missingEnumValues, 'missing-enum-values'],
      [missingEnumColor, 'missing-enum-color'],
      [missingActionDestructive, 'missing-action-destructive'],
    ] as const) {
      expect(validatePageCapabilityDefinition(capability)).toEqual(
        expect.arrayContaining([expect.objectContaining({ code })]),
      );
    }
  });

  it('rejects version 2 defaults that exceed compiled page-size and sort limits', () => {
    const capability = validV2Capability();
    capability.defaultPresentation = {
      ...capability.defaultPresentation,
      list: {
        ...capability.defaultPresentation.list,
        pageSize: 101,
        defaultSort: [
          { field: 'code', direction: 'asc' },
          { field: 'name', direction: 'desc' },
        ],
      },
    };

    expect(validatePageCapabilityDefinition(capability)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: 'page-size-exceeds-data-source-limit',
          path: 'defaultPresentation.list.pageSize',
        }),
        expect.objectContaining({
          code: 'unsupported-page-size',
          path: 'defaultPresentation.list.pageSize',
        }),
        expect.objectContaining({
          code: 'too-many-data-source-sort-fields',
          path: 'defaultPresentation.list.defaultSort',
        }),
      ]),
    );
  });

  it('enforces version 2 enum and nullability facts', () => {
    const capability = validV2Capability();
    capability.fields = capability.fields.map((field) => {
      if (field.id === 'code') {
        return { ...field, nullable: true };
      }
      if (field.id === 'country') {
        return {
          ...field,
          enumValues: [{ value: 'cn', label: { 'zh-CN': '中国', 'en-US': 'China' } }],
        };
      }
      if (field.valueType === 'enum') {
        const { enumValues: _enumValues, ...withoutEnumValues } = field;
        return withoutEnumValues;
      }
      return field;
    });

    expect(validatePageCapabilityDefinition(capability)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: 'conflicting-field-nullability', path: 'fields[0]' }),
        expect.objectContaining({ code: 'unexpected-enum-values', path: 'fields[2].enumValues' }),
        expect.objectContaining({ code: 'missing-enum-values', path: 'fields[5].enumValues' }),
      ]),
    );
  });

  it('rejects Go-only regular expressions from version 2 capabilities', () => {
    const capability = validV2Capability();
    capability.fields = capability.fields.map((field, index) =>
      index === 0
        ? {
            ...field,
            validation: { ...field.validation, pattern: '(?P<code>[A-Z]+)' },
          }
        : field,
    );

    expect(validatePageCapabilityDefinition(capability)).toEqual(
      expect.arrayContaining([expect.objectContaining({ code: 'non-portable-field-pattern' })]),
    );
  });

  it('canonicalizes hostile HTML characters as UTF-8 JSON with ordinal key ordering', () => {
    expect(
      canonicalizeCapabilityContract({
        zeta: `&<>\u2028\u2029Literal\\u2028`,
        alpha: '供应商',
      }),
    ).toBe(`{"alpha":"供应商","zeta":"&<>\u2028\u2029Literal\\\\u2028"}`);
  });

  it('uses version 2 surface component bindings for defaults and profile patches', () => {
    const capability = validV2Capability();
    capability.fields = capability.fields.map((field) =>
      field.id === 'code'
        ? {
            ...field,
            surfaceComponents: field.surfaces.map((surface) => ({
              surface,
              components: surface === 'list' || surface === 'detail' ? ['text'] : ['input'],
            })),
          }
        : field,
    );

    expect(
      validatePagePresentationProfile(
        capability,
        profile(
          'invalid-list-component',
          { kind: 'application' },
          {
            list: { columns: [{ field: 'code', component: 'input' }] },
          },
        ),
      ),
    ).toEqual(
      expect.arrayContaining([expect.objectContaining({ code: 'unsupported-field-component' })]),
    );

    capability.defaultPresentation = {
      ...capability.defaultPresentation,
      list: {
        ...capability.defaultPresentation.list,
        columns: capability.defaultPresentation.list.columns.map((field) =>
          field.field === 'code' ? { ...field, component: 'input' } : field,
        ),
      },
    };
    expect(validatePageCapabilityDefinition(capability)).toEqual(
      expect.arrayContaining([expect.objectContaining({ code: 'unsupported-field-component' })]),
    );
  });

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
          fields: [
            {
              field: 'code',
              hidden: true,
              visibleWhen: { field: 'enabled', operator: 'eq', value: true },
            },
          ],
        },
      },
    );
    Object.assign(invalid.spec, { url: 'https://example.test/data', method: 'GET' });

    const issues = validatePagePresentationProfile(supplierPresentationCapability, invalid);
    expect(issues).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ code: 'unknown-field' }),
        expect.objectContaining({ code: 'required-form-field-hidden' }),
        expect.objectContaining({ code: 'required-form-field-conditional' }),
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
