import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

export const optionPresentationRegistryEntry = corePresentationRegistry['option.list'];

export const optionPresentationListComponents = {
  name: 'option-name',
  displayName: 'text',
  category: 'code',
  status: 'option-status',
  version: 'number',
  updatedAt: 'date-time',
} as const;

export const optionPresentationSearchComponents = {
  name: 'input',
  category: 'input',
  status: 'status-filter',
} as const;

export const optionPresentationMobileFields = [
  'name',
  'displayName',
  'category',
  'status',
  'version',
] as const;
