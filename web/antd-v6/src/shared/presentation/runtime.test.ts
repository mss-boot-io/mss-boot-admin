import { corePresentationRegistry } from '@mss-admin-core/generated/core-presentation-registry.generated';
import { supplierPresentationDefinition } from '@mss-admin-core/generated/modules/supplier/presentation.generated';
import type { CurrentUser } from '@mss-admin-core/shared/auth/types';
import { describe, expect, it } from 'vitest';
import { buildPresentationConditionContext, evaluatePresentationCondition } from './condition';
import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminPagePresentationProfile,
  type PageCapabilityDefinition,
  type PagePresentationScope,
} from './contract';
import { parseEffectivePresentationResponse } from './effective';
import { createEffectivePresentationAPI, resolveEffectivePagePresentation } from './runtime';

const definition = supplierPresentationDefinition as PageCapabilityDefinition;
const entry = {
  definition,
  definitionHash: definition.definitionHash,
};
const limitedDefinition = corePresentationRegistry['user.list']
  .definition as unknown as PageCapabilityDefinition;
const limitedEntry = {
  definition: limitedDefinition,
  definitionHash: limitedDefinition.definitionHash,
};

function user(root = false): CurrentUser {
  return {
    id: 'user-1',
    role: { root },
    permissions: root
      ? {}
      : Object.fromEntries(
          [
            '/suppliers',
            '/suppliers/permissions/list',
            '/suppliers/permissions/read',
            '/suppliers/permissions/create',
            '/suppliers/permissions/update',
            '/suppliers/permissions/delete',
            '/suppliers/permissions/export',
          ].map((permission) => [permission, true]),
        ),
  };
}

function profile(
  spec: AdminPagePresentationProfile['spec'],
  scope: PagePresentationScope = { kind: 'application' },
): AdminPagePresentationProfile {
  return {
    apiVersion: ADMIN_PRESENTATION_API_VERSION,
    kind: ADMIN_PRESENTATION_KIND,
    metadata: {
      name: 'supplier-application',
      pageKey: definition.pageKey,
      definitionHash: definition.definitionHash,
      scope,
    },
    spec,
  };
}

function effective(state: 'disabled' | 'shadow' | 'active', layers: Record<string, unknown> = {}) {
  return parseEffectivePresentationResponse({
    pageKey: definition.pageKey,
    definitionHash: definition.definitionHash,
    adoption: {
      mode: state,
      state,
      resolveLayers: state !== 'disabled',
      applyLayers: state === 'active',
    },
    layers,
    diagnostics: [],
  });
}

describe('effective presentation runtime', () => {
  it('keeps compiled defaults for disabled and shadow decisions', () => {
    const layer = profile({ title: { 'en-US': 'Configured suppliers' } });
    const disabled = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response: effective('disabled', { application: layer }),
      settled: true,
    });
    const shadow = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response: effective('shadow', { application: layer }),
      settled: true,
    });

    expect(disabled.source).toBe('compiled');
    expect(disabled.model.title).toBe('Suppliers');
    expect(disabled.shadowModel).toBeUndefined();
    expect(shadow.source).toBe('compiled');
    expect(shadow.model.title).toBe('Suppliers');
    expect(shadow.shadowModel?.title).toBe('Configured suppliers');
  });

  it('applies a valid active layer across every render surface', () => {
    const response = effective('active', {
      application: profile({
        title: { 'en-US': 'Configured suppliers' },
        list: {
          density: 'compact',
          pageSize: 50,
          defaultSort: [{ field: 'name', direction: 'desc' }],
          columns: [{ field: 'country', hidden: true }],
        },
        search: { fields: [{ field: 'country', hidden: true }] },
        form: { columns: 2, fields: [{ field: 'country', hidden: true }] },
        detail: { columns: 2, fields: [{ field: 'contactEmail', hidden: true }] },
        actions: [{ action: 'supplier.export', hidden: true }],
      }),
    });
    const runtime = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response,
      settled: true,
    });

    expect(runtime.source).toBe('active');
    expect(runtime.model.title).toBe('Configured suppliers');
    expect(runtime.model.list.density).toBe('compact');
    expect(runtime.model.list.pageSize).toBe(50);
    expect(runtime.model.list.defaultSort).toEqual([{ field: 'name', direction: 'desc' }]);
    expect(runtime.model.list.columns.map((field) => field.field)).not.toContain('country');
    expect(runtime.model.search.fields.map((field) => field.field)).not.toContain('country');
    expect(runtime.model.form.columns).toBe(2);
    expect(runtime.model.form.fields.map((field) => field.field)).not.toContain('country');
    expect(runtime.model.detail.columns).toBe(2);
    expect(runtime.model.detail.fields.map((field) => field.field)).not.toContain('contactEmail');
    expect(runtime.model.actions.map((action) => action.action)).not.toContain('supplier.export');
  });

  it('merges application, role, and user layers in deterministic precedence order', () => {
    const runtime = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response: effective('active', {
        application: profile({
          title: { 'en-US': 'Application suppliers' },
          list: { density: 'compact' },
        }),
        role: profile(
          {
            title: { 'en-US': 'Role suppliers' },
            list: { pageSize: 50 },
          },
          { kind: 'role', subject: 'role-1' },
        ),
        user: profile(
          { title: { 'en-US': 'Personal suppliers' } },
          { kind: 'user', subject: 'user-1' },
        ),
      }),
      settled: true,
    });

    expect(runtime.source).toBe('active');
    expect(runtime.model.title).toBe('Personal suppliers');
    expect(runtime.model.list.density).toBe('compact');
    expect(runtime.model.list.pageSize).toBe(50);
  });

  it('merges localized title keys across layers before applying locale fallback', () => {
    const response = effective('active', {
      application: profile({ title: { 'en-US': 'Application suppliers' } }),
      role: profile({ title: { 'zh-CN': '角色供应商' } }, { kind: 'role', subject: 'role-1' }),
    });
    const english = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response,
      settled: true,
    });
    const chinese = resolveEffectivePagePresentation({
      entry,
      locale: 'zh-CN',
      user: user(),
      response,
      settled: true,
    });

    expect(english.model.title).toBe('Application suppliers');
    expect(chinese.model.title).toBe('角色供应商');
  });

  it('fails closed for mismatched identity and intersects action permissions last', () => {
    const mismatched = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response: { ...effective('active'), definitionHash: `sha256:${'0'.repeat(64)}` },
      settled: true,
    });
    const limited = user();
    limited.permissions = {
      '/suppliers': true,
      '/suppliers/permissions/list': true,
      '/suppliers/permissions/export': false,
    };
    const secured = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: limited,
      response: effective('active', {
        application: profile({ actions: [{ action: 'supplier.export', hidden: false }] }),
      }),
      settled: true,
    });

    expect(mismatched.source).toBe('compiled');
    expect(mismatched.model.title).toBe('Suppliers');
    expect(secured.model.status).toBe('ready');
    expect(secured.model.actions).toEqual([]);
  });

  it('fails closed when a published active layer adds a condition to a limited core page', () => {
    const conditionalProfile: AdminPagePresentationProfile = {
      apiVersion: ADMIN_PRESENTATION_API_VERSION,
      kind: ADMIN_PRESENTATION_KIND,
      metadata: {
        name: 'user-application',
        pageKey: limitedDefinition.pageKey,
        definitionHash: limitedDefinition.definitionHash,
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
    const response = parseEffectivePresentationResponse({
      pageKey: limitedDefinition.pageKey,
      definitionHash: limitedDefinition.definitionHash,
      adoption: {
        mode: 'active',
        state: 'active',
        resolveLayers: true,
        applyLayers: true,
      },
      layers: { application: conditionalProfile },
      diagnostics: [],
    });

    const runtime = resolveEffectivePagePresentation({
      entry: limitedEntry,
      locale: 'en-US',
      user: {
        id: 'user-1',
        role: { root: false },
        permissions: { '/users': true },
      },
      response,
      settled: true,
    });

    expect(runtime.source).toBe('compiled');
    expect(
      runtime.model.search.fields.find((field) => field.field === 'name')?.visibleWhen,
    ).toBeUndefined();
    expect(runtime.diagnostics).toContainEqual({
      code: 'runtime-layer-rejected',
      layer: 'application',
    });
  });

  it('parses lower-camel field, sort, and nested condition references from effective layers', () => {
    const response = effective('active', {
      application: profile({
        dataSource: 'supplier.list',
        list: {
          columns: [{ field: 'contactEmail', component: 'text' }],
          defaultSort: [{ field: 'createdAt', direction: 'desc' }],
        },
        detail: {
          fields: [
            {
              field: 'updatedAt',
              component: 'date-time',
              visibleWhen: {
                all: [
                  { field: 'creditLevel', operator: 'eq', value: 'a' },
                  { field: 'contactName', operator: 'exists' },
                ],
              },
            },
          ],
        },
        actions: [{ action: 'supplier.read', placement: 'row' }],
      }),
    });

    expect(response.layers.application?.spec.list?.columns?.[0]?.field).toBe('contactEmail');
    expect(response.layers.application?.spec.list?.defaultSort?.[0]?.field).toBe('createdAt');
    expect(response.layers.application?.spec.detail?.fields?.[0]?.visibleWhen).toEqual({
      all: [
        { field: 'creditLevel', operator: 'eq', value: 'a' },
        { field: 'contactName', operator: 'exists' },
      ],
    });

    const runtime = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      user: user(),
      response: effective('active', {
        application: profile({
          list: { columns: [{ field: 'contactEmail', hidden: true }] },
          detail: {
            fields: [
              {
                field: 'updatedAt',
                visibleWhen: { field: 'creditLevel', operator: 'eq', value: 'a' },
              },
            ],
          },
        }),
      }),
      settled: true,
    });
    expect(runtime.source).toBe('active');
    expect(runtime.model.list.columns.map((field) => field.field)).not.toContain('contactEmail');
    expect(runtime.model.detail.fields.find((field) => field.field === 'updatedAt')).toMatchObject({
      visibleWhen: { field: 'creditLevel', operator: 'eq', value: 'a' },
    });
  });

  it('rejects malformed profile field references without relaxing other identifiers', () => {
    const invalidSpecs: readonly AdminPagePresentationProfile['spec'][] = [
      { list: { columns: [{ field: 'ContactEmail' }] } },
      { list: { defaultSort: [{ field: 'contact@email', direction: 'asc' }] } },
      {
        detail: {
          fields: [
            {
              field: 'contactEmail',
              visibleWhen: { field: 'contact:email', operator: 'exists' },
            },
          ],
        },
      },
      { dataSource: 'supplierList' },
      { list: { columns: [{ field: 'contactEmail', component: 'Text' }] } },
      { actions: [{ action: 'supplier.Read' }] },
    ];

    for (const spec of invalidSpecs) {
      expect(() => effective('active', { application: profile(spec) })).toThrow(
        'Invalid effective presentation response',
      );
    }
    expect(() =>
      parseEffectivePresentationResponse({
        pageKey: 'supplierList',
        definitionHash: definition.definitionHash,
        adoption: {
          mode: 'active',
          state: 'active',
          resolveLayers: true,
          applyLayers: true,
        },
        layers: {},
        diagnostics: [],
      }),
    ).toThrow('Invalid effective presentation response');
  });

  it('rejects impossible adoption decisions', () => {
    expect(() =>
      parseEffectivePresentationResponse({
        pageKey: definition.pageKey,
        definitionHash: definition.definitionHash,
        adoption: { mode: 'shadow', state: 'shadow', resolveLayers: true, applyLayers: true },
        layers: {},
        diagnostics: [],
      }),
    ).toThrow('Invalid effective presentation response');
  });

  it('aborts a bounded effective request without retrying or blocking the caller', async () => {
    let attempts = 0;
    const api = createEffectivePresentationAPI(
      (_path, options) =>
        new Promise((_resolve, reject) => {
          attempts += 1;
          options.signal.addEventListener('abort', () =>
            reject(new DOMException('aborted', 'AbortError')),
          );
        }),
    );

    await expect(
      api.load(definition.pageKey, new AbortController().signal, 1),
    ).rejects.toMatchObject({
      name: 'AbortError',
    });
    expect(attempts).toBe(1);
  });
});

describe('presentation condition evaluator', () => {
  it('distinguishes missing and null and never coerces values', () => {
    const searchValues = buildPresentationConditionContext(
      { q: 'SUP', enabled: 'all' },
      { code: 'q' },
    );
    expect(searchValues).toMatchObject({ q: 'SUP', code: 'SUP', enabled: 'all' });
    expect(
      evaluatePresentationCondition(
        { field: 'code', operator: 'eq', value: 'SUP' },
        searchValues,
        definition,
      ),
    ).toBe(true);
    expect(
      evaluatePresentationCondition(
        { field: 'country', operator: 'exists' },
        { country: null },
        definition,
      ),
    ).toBe(true);
    expect(
      evaluatePresentationCondition({ field: 'country', operator: 'not-exists' }, {}, definition),
    ).toBe(true);
    expect(
      evaluatePresentationCondition(
        { field: 'enabled', operator: 'eq', value: true },
        { enabled: 'true' },
        definition,
      ),
    ).toBe(false);
    expect(
      evaluatePresentationCondition(
        { field: 'enabled', operator: 'in', value: [true] },
        { enabled: true },
        definition,
      ),
    ).toBe(true);
  });
});
