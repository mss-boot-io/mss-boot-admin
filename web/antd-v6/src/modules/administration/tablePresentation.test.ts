import type { PageCapabilityField } from '@mss-admin-core/shared/presentation/contract';
import { validatePageCapabilityDefinition } from '@mss-admin-core/shared/presentation/contract';
import { describe, expect, it } from 'vitest';
import {
  departmentPresentationListComponents,
  departmentPresentationRegistryEntry,
  departmentPresentationSearchComponents,
  menuPresentationListComponents,
  menuPresentationRegistryEntry,
  menuPresentationSearchComponents,
  postPresentationListComponents,
  postPresentationRegistryEntry,
  postPresentationSearchComponents,
  rolePresentationListComponents,
  rolePresentationRegistryEntry,
  rolePresentationSearchComponents,
} from './tablePresentation';

const cases = [
  {
    entry: rolePresentationRegistryEntry,
    listComponents: rolePresentationListComponents,
    pageKey: 'role.list',
    searchComponents: rolePresentationSearchComponents,
  },
  {
    entry: menuPresentationRegistryEntry,
    listComponents: menuPresentationListComponents,
    pageKey: 'menu.list',
    searchComponents: menuPresentationSearchComponents,
  },
  {
    entry: departmentPresentationRegistryEntry,
    listComponents: departmentPresentationListComponents,
    pageKey: 'department.list',
    searchComponents: departmentPresentationSearchComponents,
  },
  {
    entry: postPresentationRegistryEntry,
    listComponents: postPresentationListComponents,
    pageKey: 'post.list',
    searchComponents: postPresentationSearchComponents,
  },
] as const;

describe('administration table presentation bindings', () => {
  for (const testCase of cases) {
    it(`binds ${testCase.pageKey} to exact compiled renderers and no mutable surfaces`, () => {
      const definition = testCase.entry.definition;
      const fields = definition.fields as readonly PageCapabilityField[];

      expect(validatePageCapabilityDefinition(definition)).toEqual([]);
      expect(definition.pageKey).toBe(testCase.pageKey);
      expect(testCase.entry.definitionHash).toBe(definition.definitionHash);
      expect(definition.defaultPresentation.list.columns.map((field) => field.field)).toEqual(
        Object.keys(testCase.listComponents),
      );
      expect(definition.defaultPresentation.search.fields.map((field) => field.field)).toEqual(
        Object.keys(testCase.searchComponents),
      );
      expect(definition.defaultPresentation.form.fields).toEqual([]);
      expect(definition.defaultPresentation.detail.fields).toEqual([]);
      expect(definition.defaultPresentation.actions).toEqual([]);

      for (const [field, component] of Object.entries(testCase.listComponents)) {
        expect(
          fields
            .find((candidate) => candidate.id === field)
            ?.surfaceComponents?.find((candidate) => candidate.surface === 'list')?.components,
        ).toEqual([component]);
      }
      for (const [field, component] of Object.entries(testCase.searchComponents)) {
        expect(
          fields
            .find((candidate) => candidate.id === field)
            ?.surfaceComponents?.find((candidate) => candidate.surface === 'search')?.components,
        ).toEqual([component]);
      }
    });
  }
});
