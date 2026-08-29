import {
  ADMIN_PRESENTATION_API_VERSION,
  ADMIN_PRESENTATION_KIND,
  type PageCapabilityField,
  validatePageCapabilityDefinition,
} from '@mss-admin-core/shared/presentation/contract';
import { resolveEffectivePagePresentation } from '@mss-admin-core/shared/presentation/runtime';
import { describe, expect, it } from 'vitest';
import {
  userPresentationListComponents,
  userPresentationRegistryEntry,
  userPresentationSearchComponents,
} from './userPresentation';

const rootUser = {
  id: 'root-user',
  role: { root: true },
  permissions: {},
};

describe('user presentation adapter', () => {
  it('binds every generated field to one compiled renderer and keeps sensitive surfaces empty', () => {
    const definition = userPresentationRegistryEntry.definition;
    const fields = definition.fields as readonly PageCapabilityField[];

    expect(validatePageCapabilityDefinition(definition)).toEqual([]);
    expect(definition.pageKey).toBe('user.list');
    expect(definition.defaultPresentation.form.fields).toEqual([]);
    expect(definition.defaultPresentation.detail.fields).toEqual([]);
    expect(definition.defaultPresentation.actions).toEqual([]);
    const fieldIDs = fields.map((field) => field.id);
    expect(fieldIDs).toHaveLength(6);
    expect(fieldIDs).toEqual(
      expect.arrayContaining(['username', 'name', 'email', 'roleName', 'organization', 'status']),
    );
    expect(fieldIDs).not.toContain('password');
    expect(fieldIDs).not.toContain('confirmPassword');

    for (const [field, component] of Object.entries(userPresentationListComponents)) {
      expect(
        fields
          .find((candidate) => candidate.id === field)
          ?.surfaceComponents?.find((candidate) => candidate.surface === 'list')?.components,
      ).toEqual([component]);
    }
    for (const [field, component] of Object.entries(userPresentationSearchComponents)) {
      expect(
        fields
          .find((candidate) => candidate.id === field)
          ?.surfaceComponents?.find((candidate) => candidate.surface === 'search')?.components,
      ).toEqual([component]);
    }
  });

  it('keeps compiled defaults without a profile and applies only generated active presentation facts', () => {
    const compiled = resolveEffectivePagePresentation({
      entry: userPresentationRegistryEntry,
      locale: 'en-US',
      user: rootUser,
      settled: false,
    });

    expect(compiled.source).toBe('compiled');
    expect(compiled.model.title).toBe('Users');
    expect(compiled.model.list.columns.map((field) => field.field)).toEqual([
      'username',
      'name',
      'email',
      'roleName',
      'organization',
      'status',
    ]);
    expect(compiled.model.list).toMatchObject({ density: 'large', pageSize: 20 });
    expect(compiled.model.search).toMatchObject({ collapsedByDefault: false });
    expect(compiled.model.search.fields.map((field) => field.field)).toEqual(['name', 'status']);

    const definitionHash = userPresentationRegistryEntry.definitionHash;
    const active = resolveEffectivePagePresentation({
      entry: userPresentationRegistryEntry,
      locale: 'en-US',
      user: rootUser,
      settled: true,
      response: {
        pageKey: 'user.list',
        definitionHash,
        adoption: {
          mode: 'active',
          state: 'active',
          resolveLayers: true,
          applyLayers: true,
        },
        diagnostics: [],
        layers: {
          application: {
            apiVersion: ADMIN_PRESENTATION_API_VERSION,
            kind: ADMIN_PRESENTATION_KIND,
            metadata: {
              name: 'user-list-active-test',
              pageKey: 'user.list',
              definitionHash,
              scope: { kind: 'application' },
            },
            spec: {
              title: { 'en-US': 'Directory operators' },
              list: {
                columns: [
                  { field: 'email', hidden: true },
                  {
                    field: 'status',
                    order: 5,
                    label: { 'en-US': 'Account state' },
                  },
                ],
                density: 'compact',
                pageSize: 50,
              },
              search: {
                collapsedByDefault: true,
                fields: [
                  { field: 'name', label: { 'en-US': 'Display name' } },
                  { field: 'status', hidden: true },
                ],
              },
            },
          },
        },
      },
    });

    expect(active.source).toBe('active');
    expect(active.model.title).toBe('Directory operators');
    expect(active.model.list.columns.map((field) => field.field)).not.toContain('email');
    expect(active.model.list.columns[0]).toMatchObject({
      field: 'status',
      label: 'Account state',
    });
    expect(active.model.list).toMatchObject({ density: 'compact', pageSize: 50 });
    expect(active.model.search.fields).toEqual([
      expect.objectContaining({ field: 'name', label: 'Display name' }),
    ]);
    expect(active.model.search.collapsedByDefault).toBe(true);
    expect(active.model.form.fields).toEqual([]);
    expect(active.model.detail.fields).toEqual([]);
    expect(active.model.actions).toEqual([]);
  });
});
