import { corePresentationRegistry } from '../../generated/core-presentation-registry.generated';

export const rolePresentationRegistryEntry = corePresentationRegistry['role.list'];
export const menuPresentationRegistryEntry = corePresentationRegistry['menu.list'];
export const departmentPresentationRegistryEntry = corePresentationRegistry['department.list'];
export const postPresentationRegistryEntry = corePresentationRegistry['post.list'];

export const rolePresentationListComponents = {
  name: 'text',
  classification: 'role-classification',
  status: 'status-tag',
  remark: 'text',
} as const;

export const rolePresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const rolePresentationMobileFields = ['name', 'classification', 'status'] as const;

export const menuPresentationListComponents = {
  name: 'menu-label',
  path: 'code',
  type: 'menu-type',
  permission: 'text',
  status: 'status-tag',
} as const;

export const menuPresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const menuPresentationMobileFields = ['name', 'type', 'path', 'status'] as const;

export const departmentPresentationListComponents = {
  name: 'text',
  code: 'code',
  leaderID: 'department-leader',
  contact: 'department-contact',
  status: 'status-tag',
  sort: 'number',
} as const;

export const departmentPresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const departmentPresentationMobileFields = ['name', 'code', 'leaderID', 'status'] as const;

export const postPresentationListComponents = {
  name: 'text',
  code: 'code',
  dataScope: 'post-data-scope',
  status: 'status-tag',
  sort: 'number',
} as const;

export const postPresentationSearchComponents = {
  name: 'input',
  status: 'status-filter',
} as const;

export const postPresentationMobileFields = ['name', 'code', 'dataScope', 'status'] as const;
