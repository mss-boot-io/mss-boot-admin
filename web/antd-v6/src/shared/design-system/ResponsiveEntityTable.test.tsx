import { fireEvent, render, screen } from '@testing-library/react';
import type { TableColumnsType } from 'antd';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import ResponsiveEntityTable from './ResponsiveEntityTable';

const viewport = vi.hoisted(() => ({ md: false }));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    Grid: {
      ...actual.Grid,
      useBreakpoint: () => ({ md: viewport.md }),
    },
  };
});

interface Row {
  id: string;
  name: string;
  status: string;
}

const edit = vi.fn();
const columns: TableColumnsType<Row> = [
  { dataIndex: 'name', key: 'name', title: '名称' },
  {
    dataIndex: 'status',
    key: 'status',
    title: '状态',
    render: (value) => <strong>{value === 'enabled' ? '启用' : '停用'}</strong>,
  },
  {
    key: 'actions',
    title: '操作',
    render: (_, row) => (
      <button type="button" onClick={() => edit(row.id)}>
        编辑
      </button>
    ),
  },
];

beforeEach(() => {
  edit.mockReset();
  viewport.md = false;
});

describe('ResponsiveEntityTable', () => {
  it('renders the selected fields and row actions as a mobile card', () => {
    render(
      <ResponsiveEntityTable<Row>
        columns={columns}
        dataSource={[{ id: 'row-1', name: '总部', status: 'enabled' }]}
        mobileColumnKeys={['name', 'status', 'actions']}
        pagination={false}
        rowKey="id"
      />,
    );

    expect(screen.queryByRole('columnheader')).toBeNull();
    expect(screen.getByText('总部')).toBeTruthy();
    expect(screen.getByText('启用')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    expect(edit).toHaveBeenCalledWith('row-1');
  });

  it('keeps the Ant Design table on desktop', () => {
    viewport.md = true;

    render(
      <ResponsiveEntityTable<Row>
        columns={columns}
        dataSource={[{ id: 'row-1', name: '总部', status: 'enabled' }]}
        mobileColumnKeys={['name']}
        pagination={false}
        rowKey="id"
      />,
    );

    expect(screen.getByRole('columnheader', { name: '名称' })).toBeTruthy();
  });
});
