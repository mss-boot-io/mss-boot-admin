import type { PageCapabilityField } from '@mss-admin-core/shared/presentation/contract';
import { validatePageCapabilityDefinition } from '@mss-admin-core/shared/presentation/contract';
import { describe, expect, it } from 'vitest';
import {
  auditLogPresentationListComponents,
  auditLogPresentationRegistryEntry,
  auditLogPresentationSearchComponents,
  loginLogPresentationListComponents,
  loginLogPresentationRegistryEntry,
  loginLogPresentationSearchComponents,
  noticePresentationListComponents,
  noticePresentationRegistryEntry,
  noticePresentationSearchComponents,
  runtimeLogPresentationListComponents,
  runtimeLogPresentationRegistryEntry,
  runtimeLogPresentationSearchComponents,
  systemConfigPresentationListComponents,
  systemConfigPresentationRegistryEntry,
  systemConfigPresentationSearchComponents,
  taskPresentationListComponents,
  taskPresentationRegistryEntry,
  taskPresentationSearchComponents,
} from './tablePresentation';

const cases = [
  {
    entry: taskPresentationRegistryEntry,
    listComponents: taskPresentationListComponents,
    pageKey: 'task.list',
    searchComponents: taskPresentationSearchComponents,
  },
  {
    entry: noticePresentationRegistryEntry,
    listComponents: noticePresentationListComponents,
    pageKey: 'notice.list',
    searchComponents: noticePresentationSearchComponents,
  },
  {
    entry: systemConfigPresentationRegistryEntry,
    listComponents: systemConfigPresentationListComponents,
    pageKey: 'system-config.list',
    searchComponents: systemConfigPresentationSearchComponents,
  },
  {
    entry: loginLogPresentationRegistryEntry,
    listComponents: loginLogPresentationListComponents,
    pageKey: 'log.login',
    searchComponents: loginLogPresentationSearchComponents,
  },
  {
    entry: auditLogPresentationRegistryEntry,
    listComponents: auditLogPresentationListComponents,
    pageKey: 'log.audit',
    searchComponents: auditLogPresentationSearchComponents,
  },
  {
    entry: runtimeLogPresentationRegistryEntry,
    listComponents: runtimeLogPresentationListComponents,
    pageKey: 'log.runtime',
    searchComponents: runtimeLogPresentationSearchComponents,
  },
] as const;

describe('operations table presentation bindings', () => {
  for (const testCase of cases) {
    it(`binds ${testCase.pageKey} to exact compiled renderers and no mutable actions`, () => {
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
