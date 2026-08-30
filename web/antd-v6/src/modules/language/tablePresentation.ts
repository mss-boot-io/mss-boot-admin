import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

export const languagePresentationRegistryEntry = corePresentationRegistry['language.list'];

export const languagePresentationListComponents = {
  name: 'language-name',
  status: 'language-status',
  remark: 'text',
  updatedAt: 'date-time',
} as const;

export const languagePresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const languagePresentationMobileFields = ['name', 'status', 'remark'] as const;
