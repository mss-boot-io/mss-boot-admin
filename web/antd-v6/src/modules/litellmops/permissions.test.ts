import type { CurrentUser } from '@mss-admin-core/shared/auth/types';
import { describe, expect, it } from 'vitest';
import { buildOpsAccess, opsPermissions } from './permissions';

describe('LiteLLM operations action permissions', () => {
  it('keeps read-only resources separate from management recovery', () => {
    const reader = {
      id: 'reader',
      role: { root: false },
      permissions: {
        [opsPermissions.orgList]: true,
        [opsPermissions.orgRead]: true,
        [opsPermissions.managementResolve]: false,
      },
    } as CurrentUser;

    expect(buildOpsAccess(reader)).toMatchObject({
      canReadOrganizations: true,
      canReadOrganizationDetails: true,
      canResolveManagement: false,
    });
  });
});
