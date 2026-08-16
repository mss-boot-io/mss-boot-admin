import type { TableColumnsType, TableProps } from 'antd';
import { Card, Descriptions, Grid, List, Table } from 'antd';
import { isValidElement, type Key, type ReactNode } from 'react';

interface ResponsiveEntityTableProps<T extends object> extends TableProps<T> {
  mobileColumnKeys: readonly string[];
}

function columnKey<T>(column: TableColumnsType<T>[number]): string | undefined {
  if (column.key !== undefined) return String(column.key);
  if (!('dataIndex' in column) || column.dataIndex === undefined) return undefined;
  return Array.isArray(column.dataIndex)
    ? column.dataIndex.map(String).join('.')
    : String(column.dataIndex);
}

function dataIndexValue<T>(record: T, dataIndex: unknown): unknown {
  const path = Array.isArray(dataIndex) ? dataIndex : [dataIndex];
  return path.reduce<unknown>((value, segment) => {
    if (!value || typeof value !== 'object') return undefined;
    return (value as Record<PropertyKey, unknown>)[segment as PropertyKey];
  }, record);
}

function renderedNode(value: unknown): ReactNode {
  if (value && typeof value === 'object' && !isValidElement(value) && 'children' in value) {
    return (value as { children?: ReactNode }).children;
  }
  return value as ReactNode;
}

function descriptionItems<T extends object>(
  columns: TableColumnsType<T>,
  keys: readonly string[],
  record: T,
  index: number,
) {
  const selected = new Set(keys);
  return columns.flatMap((column) => {
    const key = columnKey(column);
    if (!key || !selected.has(key) || 'children' in column) return [];
    const value = dataIndexValue(record, 'dataIndex' in column ? column.dataIndex : undefined);
    const rendered: ReactNode =
      'render' in column && typeof column.render === 'function'
        ? renderedNode(column.render(value, record, index))
        : ((value ?? '—') as ReactNode);
    return [
      {
        key,
        label: typeof column.title === 'function' ? key : column.title,
        children: rendered,
      },
    ];
  });
}

export default function ResponsiveEntityTable<T extends object>({
  mobileColumnKeys,
  ...tableProps
}: ResponsiveEntityTableProps<T>) {
  const screens = Grid.useBreakpoint();
  if (screens.md !== false) return <Table<T> {...tableProps} />;

  const pagination =
    tableProps.pagination && typeof tableProps.pagination === 'object'
      ? tableProps.pagination
      : undefined;
  const emptyText =
    typeof tableProps.locale?.emptyText === 'function'
      ? tableProps.locale.emptyText()
      : tableProps.locale?.emptyText;

  return (
    <List<T>
      dataSource={[...(tableProps.dataSource ?? [])]}
      loading={tableProps.loading}
      locale={{ emptyText }}
      pagination={
        pagination
          ? {
              current: pagination.current,
              hideOnSinglePage: true,
              pageSize: pagination.pageSize,
              simple: true,
              total: pagination.total,
              onChange: (page, pageSize) => pagination.onChange?.(page, pageSize),
            }
          : false
      }
      rowKey={tableProps.rowKey as keyof T | ((item: T) => Key)}
      renderItem={(item, index) => (
        <List.Item style={{ paddingInline: 0 }}>
          <Card className="w-full" size="small">
            <Descriptions
              column={1}
              items={descriptionItems(tableProps.columns ?? [], mobileColumnKeys, item, index)}
              size="small"
            />
          </Card>
        </List.Item>
      )}
    />
  );
}
