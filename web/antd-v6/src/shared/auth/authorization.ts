import { request } from '@umijs/max';
import { retainRegisteredMenu } from '../routes/registry';
import type { AuthorizedMenuItem, CurrentUser } from './types';

export class AuthorizationMenuContractError extends Error {
  constructor() {
    super('Authorized menu response must be an array');
    this.name = 'AuthorizationMenuContractError';
  }
}

export async function fetchAuthorizedMenu(user: CurrentUser): Promise<AuthorizedMenuItem[]> {
  const value = await request<unknown>('/menu/authorize', {
    method: 'GET',
    skipErrorHandler: true,
  });
  if (!Array.isArray(value)) throw new AuthorizationMenuContractError();
  return retainRegisteredMenu(value, user);
}
