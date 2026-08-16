import { describe, expect, it } from 'vitest';
import {
  AdministrationContractError,
  administrationReferenceName,
  administrationSelectOptions,
  administrationSubtreeIDs,
  parseAdministrationPage,
  parseDepartment,
  parseRole,
  serializeRoleWrite,
  serializeUserWrite,
} from './contract';

const params = { current: 1, pageSize: 20 } as const;

describe('administration contracts', () => {
  it('never uses a relation identifier as display text', () => {
    const names = new Map([['role-1', 'Administrator']]);

    expect(administrationReferenceName(undefined, 'role-1', names)).toBe('Administrator');
    expect(administrationReferenceName({ name: 'Embedded role' }, 'role-1', names)).toBe(
      'Embedded role',
    );
    expect(administrationReferenceName(undefined, 'unresolved-id', names)).toBe('—');
  });

  it('parses bounded page and managed-role flags without trusting extra fields', () => {
    const page = parseAdministrationPage(
      {
        data: [
          {
            id: 'role-1',
            name: 'Operators',
            remark: 'bounded',
            root: false,
            default: true,
            status: 'enabled',
            secret: 'ignored',
          },
        ],
        total: 1,
        current: 1,
        pageSize: 20,
      },
      params,
      parseRole,
    );

    expect(page.data).toEqual([
      {
        id: 'role-1',
        name: 'Operators',
        remark: 'bounded',
        root: false,
        default: true,
        status: 'enabled',
        updatedAt: undefined,
      },
    ]);
    expect(JSON.stringify(page)).not.toContain('secret');
  });

  it('rejects oversized pages and cyclic tree projections', () => {
    expect(() =>
      parseAdministrationPage(
        { data: Array.from({ length: 101 }, () => ({})), total: 101 },
        params,
        parseRole,
      ),
    ).toThrow(AdministrationContractError);

    expect(() =>
      parseDepartment({
        id: 'dept-1',
        name: 'Root',
        code: 'ROOT',
        status: 'enabled',
        children: [
          {
            id: 'dept-1',
            name: 'Cycle',
            code: 'CYCLE',
            status: 'enabled',
          },
        ],
      }),
    ).toThrow(/cycle/);
  });

  it('serializes only writable authority fields and never carries edit passwords', () => {
    expect(
      serializeRoleWrite({ name: '  Operators ', remark: ' safe ', status: 'enabled' }),
    ).toEqual({ name: 'Operators', remark: 'safe', status: 'enabled' });

    const edit = serializeUserWrite(
      {
        username: 'operator_1',
        email: ' operator@example.com ',
        password: 'ShouldNotLeaveTheBrowser123',
        roleID: 'role-1',
        status: 'enabled',
      },
      'edit',
    );
    expect(edit).not.toHaveProperty('password');
    expect(edit).toMatchObject({
      username: 'operator_1',
      email: 'operator@example.com',
      roleID: 'role-1',
    });
  });

  it('flattens bounded trees into searchable path options without enabling blocked nodes', () => {
    const tree = parseDepartment({
      id: 'dept-root',
      name: 'Platform',
      code: 'PLATFORM',
      status: 'enabled',
      children: [
        {
          id: 'dept-child',
          name: 'Security',
          code: 'SECURITY',
          status: 'enabled',
        },
      ],
    });

    expect(administrationSelectOptions([tree], { disabled: new Set(['dept-child']) })).toEqual([
      { value: 'dept-root', label: 'Platform', disabled: false },
      { value: 'dept-child', label: 'Platform / Security', disabled: true },
    ]);
    expect(administrationSubtreeIDs([tree], 'dept-root')).toEqual(
      new Set(['dept-root', 'dept-child']),
    );
    expect(
      administrationSelectOptions([tree], {
        label: (department) => `localized:${department.name}`,
      }),
    ).toEqual([
      { value: 'dept-root', label: 'localized:Platform', disabled: false },
      {
        value: 'dept-child',
        label: 'localized:Platform / localized:Security',
        disabled: false,
      },
    ]);
  });
});
