import { describe, expect, it, vi } from 'vitest';
import type { AdministrationListParams, AdministrationPage, DepartmentSummary } from './contract';
import { AdministrationContractError, MAX_ADMIN_TREE_NODES } from './contract';
import { collectAdministrationCatalog } from './query';

function department(id: string, children?: DepartmentSummary[]): DepartmentSummary {
  return {
    id,
    name: id,
    parentID: '',
    status: 'enabled',
    sort: 0,
    code: id,
    leaderID: '',
    phone: '',
    email: '',
    children,
  };
}

describe('administration dependency catalogs', () => {
  it('loads every unfiltered page independently from table pagination', async () => {
    const list = vi
      .fn<(params: AdministrationListParams) => Promise<AdministrationPage<DepartmentSummary>>>()
      .mockResolvedValueOnce({ data: [department('root-1')], total: 2, current: 1, pageSize: 100 })
      .mockResolvedValueOnce({ data: [department('root-2')], total: 2, current: 2, pageSize: 100 });

    await expect(collectAdministrationCatalog(list)).resolves.toEqual([
      department('root-1'),
      department('root-2'),
    ]);
    expect(list).toHaveBeenNthCalledWith(1, { current: 1, pageSize: 100, status: 'all' });
    expect(list).toHaveBeenNthCalledWith(2, { current: 2, pageSize: 100, status: 'all' });
  });

  it('rejects incomplete, duplicate, and oversized catalogs instead of hiding choices', async () => {
    await expect(
      collectAdministrationCatalog(async () => ({
        data: [],
        total: 1,
        current: 1,
        pageSize: 100,
      })),
    ).rejects.toThrow(AdministrationContractError);

    await expect(
      collectAdministrationCatalog(async () => ({
        data: [department('root', [department('duplicate')]), department('duplicate')],
        total: 2,
        current: 1,
        pageSize: 100,
      })),
    ).rejects.toThrow(/duplicate/);

    await expect(
      collectAdministrationCatalog(async () => ({
        data: [],
        total: MAX_ADMIN_TREE_NODES + 1,
        current: 1,
        pageSize: 100,
      })),
    ).rejects.toThrow(/limit/);
  });
});
