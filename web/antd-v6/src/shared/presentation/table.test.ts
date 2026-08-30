import type { PageRenderModel } from '@mss-admin-core/shared/presentation/contract';
import {
  resolveTablePresentation,
  usePresentationSearchExpansion,
} from '@mss-admin-core/shared/presentation/table';
import { act, renderHook } from '@testing-library/react';
import type { TableColumnsType } from 'antd';
import { describe, expect, it } from 'vitest';

interface RecordFixture {
  id: string;
  name: string;
  status: string;
}

const protectedActionColumn: TableColumnsType<RecordFixture>[number] = {
  key: 'actions',
  title: 'Compiled actions',
  width: 240,
  render: () => 'compiled mutation controls',
};

const compiledColumns: TableColumnsType<RecordFixture> = [
  { dataIndex: 'name', title: 'Compiled name', width: 180 },
  { dataIndex: 'status', title: 'Compiled status', width: 120 },
  protectedActionColumn,
];

const isFixturePageSize = (value: number): value is 20 | 50 | 100 =>
  value === 20 || value === 50 || value === 100;

function model(overrides: Partial<PageRenderModel> = {}): PageRenderModel {
  return {
    pageKey: 'fixture.list',
    status: 'ready',
    title: 'Fixture',
    dataSource: 'fixture.list',
    list: {
      columns: [
        { field: 'name', component: 'text', label: 'Name', order: 10 },
        { field: 'status', component: 'status-tag', label: 'Status', order: 20 },
      ],
      density: 'large',
      pageSize: 20,
      defaultSort: [],
    },
    search: {
      fields: [
        { field: 'name', component: 'input', label: 'Name', order: 10 },
        { field: 'status', component: 'status-filter', label: 'Status', order: 20 },
      ],
      collapsedByDefault: false,
    },
    form: { fields: [], columns: 1 },
    detail: { fields: [], columns: 1 },
    actions: [],
    ...overrides,
  };
}

function columnKey(column: TableColumnsType<RecordFixture>[number]): string {
  if ('dataIndex' in column && typeof column.dataIndex === 'string') return column.dataIndex;
  return 'key' in column ? String(column.key ?? '') : '';
}

describe('table presentation adapter', () => {
  it('keeps the compiled surface and protected action column by default', () => {
    const result = resolveTablePresentation({
      compiledColumns,
      fallbackPageSize: 20,
      isPageSize: isFixturePageSize,
      listComponents: { name: 'text', status: 'status-tag' } as const,
      mobileColumnKeys: ['name', 'status', 'actions'],
      model: model(),
      protectedColumnKeys: ['actions'],
      searchComponents: { name: 'input', status: 'status-filter' } as const,
    });

    expect(result.columns.map(columnKey)).toEqual(['name', 'status', 'actions']);
    expect(result.columns[2]).toBe(protectedActionColumn);
    expect(result.mobileColumnKeys).toEqual(['name', 'status', 'actions']);
    expect(result.searchFields.get('name')).toMatchObject({ component: 'input' });
    expect(result.searchFields.get('status')).toMatchObject({ component: 'status-filter' });
    expect(result).toMatchObject({
      density: 'large',
      pageSize: 20,
      searchCollapsedByDefault: false,
    });
  });

  it('applies active visibility, order, labels, widths, filters, density, and page size', () => {
    const activeModel = model({
      list: {
        columns: [
          {
            field: 'status',
            component: 'status-tag',
            label: 'Account state',
            order: 5,
            width: 200,
          },
          // Unknown or mismatched runtime renderers must not create executable UI.
          { field: 'unknown', component: 'remote-code', label: 'Unknown', order: 6 },
        ],
        density: 'compact',
        pageSize: 50,
        defaultSort: [],
      },
      search: {
        fields: [
          { field: 'name', component: 'input', label: 'Display name', order: 20 },
          { field: 'status', component: 'wrong-filter', label: 'Ignored', order: 10 },
        ],
        collapsedByDefault: true,
      },
    });
    const result = resolveTablePresentation({
      compiledColumns,
      fallbackPageSize: 20,
      isPageSize: isFixturePageSize,
      listComponents: { name: 'text', status: 'status-tag' } as const,
      mobileColumnKeys: ['name', 'status', 'actions'],
      model: activeModel,
      protectedColumnKeys: ['actions'],
      searchComponents: { name: 'input', status: 'status-filter' } as const,
    });

    expect(result.columns.map(columnKey)).toEqual(['status', 'actions']);
    expect(result.columns[0]).toMatchObject({ title: 'Account state', width: 200 });
    expect(result.columns[1]).toBe(protectedActionColumn);
    expect(result.mobileColumnKeys).toEqual(['status', 'actions']);
    expect(result.searchFields.get('name')).toMatchObject({ label: 'Display name' });
    expect(result.searchFields.has('status')).toBe(false);
    expect(result).toMatchObject({
      density: 'compact',
      pageSize: 50,
      searchCollapsedByDefault: true,
    });
  });

  it('fails closed on an unsupported page size and a profile-owned action field', () => {
    const unsafeModel = model({
      list: {
        columns: [
          { field: 'actions', component: 'text', label: 'Profile action', order: 1, width: 999 },
          { field: 'name', component: 'text', label: 'Name', order: 2 },
        ],
        density: 'middle',
        pageSize: 999,
        defaultSort: [],
      },
    });
    const result = resolveTablePresentation({
      compiledColumns,
      fallbackPageSize: 20,
      isPageSize: isFixturePageSize,
      listComponents: { actions: 'text', name: 'text', status: 'status-tag' } as const,
      mobileColumnKeys: ['name', 'actions'],
      model: unsafeModel,
      protectedColumnKeys: ['actions'],
      searchComponents: { name: 'input', status: 'status-filter' } as const,
    });

    expect(result.columns.map(columnKey)).toEqual(['name', 'actions']);
    expect(result.columns[1]).toBe(protectedActionColumn);
    expect(result.columns[1]).toMatchObject({ title: 'Compiled actions', width: 240 });
    expect(result.pageSize).toBe(20);
  });

  it('honors the initial search collapse without overriding an operator expansion', () => {
    const view = renderHook(
      ({ collapsed }: { collapsed: boolean }) => usePresentationSearchExpansion(collapsed),
      { initialProps: { collapsed: true } },
    );
    expect(view.result.current.expanded).toBe(false);

    act(() => view.result.current.expand());
    expect(view.result.current.expanded).toBe(true);

    view.rerender({ collapsed: false });
    view.rerender({ collapsed: true });
    expect(view.result.current.expanded).toBe(true);
  });
});
