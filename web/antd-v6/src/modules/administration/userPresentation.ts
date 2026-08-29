import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

/**
 * The capability and definition hash come from the generated Foundation core
 * registry. This adapter contains only compiled view bindings; published
 * profiles can select or arrange these IDs but can never provide render code.
 */
export const userPresentationRegistryEntry = corePresentationRegistry['user.list'];

export const userPresentationListComponents = {
  username: 'user-identity',
  name: 'text',
  email: 'text',
  roleName: 'user-role',
  organization: 'user-organization',
  status: 'status-tag',
} as const;

export const userPresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const userPresentationMobileFields = ['username', 'name', 'roleName', 'status'] as const;
