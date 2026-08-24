import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { supplierPermissionPaths } from '../../generated/modules/supplier/contract';
import { canonicalizeCapabilityContract } from './contract';
import {
  buildSupplierCompactPrototype,
  supplierPresentationCapability,
  supplierPresentationCompatibility,
} from './supplier.prototype';

const allPermissions = new Set([
  '/suppliers',
  '/suppliers/permissions/list',
  '/suppliers/permissions/read',
  '/suppliers/permissions/create',
  '/suppliers/permissions/update',
  '/suppliers/permissions/delete',
  '/suppliers/permissions/export',
]);

describe('Supplier presentation P0 prototype', () => {
  it('uses the exact generated route and operation permission identifiers', () => {
    expect(supplierPresentationCompatibility.dataSources[0]?.requiredPermissions).toEqual([
      supplierPermissionPaths.route,
      supplierPermissionPaths.list,
    ]);
    expect(
      Object.fromEntries(
        supplierPresentationCompatibility.actions.map((action) => [
          action.id,
          action.requiredPermissions,
        ]),
      ),
    ).toEqual({
      'supplier.create': [supplierPermissionPaths.route, supplierPermissionPaths.create],
      'supplier.read': [supplierPermissionPaths.route, supplierPermissionPaths.read],
      'supplier.update': [supplierPermissionPaths.route, supplierPermissionPaths.update],
      'supplier.delete': [supplierPermissionPaths.route, supplierPermissionPaths.delete],
      'supplier.export': [supplierPermissionPaths.route, supplierPermissionPaths.export],
    });
  });

  it('binds the profile to the canonical trusted compatibility hash', () => {
    const digest = createHash('sha256')
      .update(canonicalizeCapabilityContract(supplierPresentationCompatibility))
      .digest('hex');

    expect(supplierPresentationCapability.definitionHash).toBe(`sha256:${digest}`);
  });

  it('projects a compact localized render model using profile data only', () => {
    const model = buildSupplierCompactPrototype(allPermissions, 'zh-CN');

    expect(model).toMatchObject({
      pageKey: 'supplier.list',
      status: 'ready',
      title: '供应商概览',
      dataSource: 'supplier.list',
      list: { density: 'compact', pageSize: 50 },
      search: { collapsedByDefault: false },
    });
    expect(model.list.columns.map((field) => [field.field, field.label])).toEqual([
      ['code', '供应商编码'],
      ['name', '供应商名称'],
      ['enabled', '状态'],
      ['creditLevel', '信用等级'],
    ]);
    expect(model.actions.map((action) => action.action)).toEqual([
      'supplier.create',
      'supplier.read',
      'supplier.update',
    ]);
  });

  it('is not imported by the production route registries', () => {
    const routeSources = [
      '../routes/registry.ts',
      '../../generated/routes.ts',
      '../../../config/routes.generated.ts',
    ].map((path) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8'));

    expect(routeSources.join('\n')).not.toContain('supplier.prototype');
  });
});
