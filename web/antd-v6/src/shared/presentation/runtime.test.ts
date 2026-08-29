import { supplierPresentationDefinition } from '@mss-admin-core/generated/modules/supplier/presentation.generated';
import type { CurrentUser } from '@mss-admin-core/shared/auth/types';
import { describe, expect, it } from 'vitest';
import { buildPresentationConditionContext, evaluatePresentationCondition } from './condition';
import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type AdminPagePresentationProfile,
  type PageCapabilityDefinition,
} from './contract';
import { parseEffectivePresentationResponse } from './effective';
import { resolveEffectivePagePresentation } from './runtime';

const definition = supplierPresentationDefinition as PageCapabilityDefinition;
const entry = {
  definition,
  definitionHash: definition.definitionHash,
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

function profile(spec: AdminPagePresentationProfile['spec']): AdminPagePresentationProfile {
  return {
    apiVersion: ADMIN_PRESENTATION_API_VERSION,
    kind: ADMIN_PRESENTATION_KIND,
    metadata: {
      name: 'supplier-application',
      pageKey: definition.pageKey,
      definitionHash: definition.definitionHash,
      scope: { kind: 'application' },
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
      response: effective('active'),
      settled: true,
    });

    expect(mismatched.source).toBe('compiled');
    expect(mismatched.model.title).toBe('Suppliers');
    expect(secured.model.status).toBe('ready');
    expect(secured.model.actions).toEqual([]);
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
