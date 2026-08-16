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
    // This request is itself the authoritative refresh. A 403 must fail it
    // closed, not recursively enqueue another refresh forever.
    skipAuthorizationRefresh: true,
  });
  if (!Array.isArray(value)) throw new AuthorizationMenuContractError();
  return retainRegisteredMenu(value, user);
}
