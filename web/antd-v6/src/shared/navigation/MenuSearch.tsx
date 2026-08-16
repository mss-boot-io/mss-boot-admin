import SearchOutlined from '@ant-design/icons/SearchOutlined';
import { history, useIntl } from '@umijs/max';
import { AutoComplete, Button, Empty, Input, Popover, Space, Typography } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import type { AuthorizedMenuItem } from '@/shared/auth/types';
import { formatMenuLabel } from './menuLocale';

const SEARCH_RESULT_LIMIT = 20;

export interface AuthorizedMenuSearchItem {
  label: string;
  path: string;
  searchText: string;
}

function normalizeSearchText(value: string): string {
  return value.normalize('NFKC').trim().toLocaleLowerCase();
}

function isNavigablePath(path: string | undefined): path is string {
  return Boolean(path?.startsWith('/') && !path.startsWith('//') && !/[:*]/.test(path));
}

export function buildAuthorizedMenuSearchItems(
  menu: readonly AuthorizedMenuItem[],
  localize: (name?: string) => string,
): AuthorizedMenuSearchItem[] {
  const result: AuthorizedMenuSearchItem[] = [];
  const seenPaths = new Set<string>();

  const visit = (nodes: readonly AuthorizedMenuItem[], parents: readonly string[]) => {
    for (const node of nodes) {
      if (node.hideInMenu) continue;
      const name = node.name ?? node.title;
      const label = localize(name) || node.path || '';
      const hierarchy = label ? [...parents, label] : [...parents];
      const navigableType = !node.type || node.type === 'MENU';
      if (navigableType && isNavigablePath(node.path) && !seenPaths.has(node.path)) {
        seenPaths.add(node.path);
        const fullLabel = hierarchy.join(' / ') || node.path;
        result.push({
          label: fullLabel,
          path: node.path,
          searchText: normalizeSearchText([fullLabel, name, node.path].filter(Boolean).join(' ')),
        });
      }
      if (!node.hideChildrenInMenu && node.children?.length) visit(node.children, hierarchy);
    }
  };

  visit(menu, []);
  return result;
}

export default function MenuSearch({ items }: { items: readonly AuthorizedMenuItem[] }) {
  const intl = useIntl();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const allItems = useMemo(
    () => buildAuthorizedMenuSearchItems(items, (name) => formatMenuLabel(intl, name)),
    [intl, items],
  );
  const results = useMemo(() => {
    const normalized = normalizeSearchText(query);
    return (
      normalized ? allItems.filter((item) => item.searchText.includes(normalized)) : allItems
    ).slice(0, SEARCH_RESULT_LIMIT);
  }, [allItems, query]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLocaleLowerCase() === 'k') {
        event.preventDefault();
        setOpen(true);
      }
      if (event.key === 'Escape') setOpen(false);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const close = () => {
    setOpen(false);
    setQuery('');
  };

  return (
    <Popover
      arrow={false}
      fresh
      open={open}
      placement="bottomRight"
      trigger="click"
      content={
        <div className="w-[min(420px,calc(100vw-32px))]">
          <AutoComplete
            autoFocus
            className="w-full"
            defaultActiveFirstOption
            value={query}
            options={results.map((item) => ({
              value: item.path,
              label: (
                <Space orientation="vertical" size={0}>
                  <Typography.Text>{item.label}</Typography.Text>
                  <Typography.Text type="secondary">{item.path}</Typography.Text>
                </Space>
              ),
            }))}
            filterOption={false}
            notFoundContent={
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={intl.formatMessage({ id: 'navigation.search.empty' })}
              />
            }
            onChange={setQuery}
            onSelect={(path) => {
              close();
              history.push(path);
            }}
          >
            <Input
              allowClear
              prefix={<SearchOutlined />}
              placeholder={intl.formatMessage({ id: 'navigation.search.placeholder' })}
              aria-label={intl.formatMessage({ id: 'navigation.search.placeholder' })}
            />
          </AutoComplete>
          <Typography.Text className="mt-2 block text-xs" type="secondary">
            {intl.formatMessage({ id: 'navigation.search.hint' })}
          </Typography.Text>
        </div>
      }
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery('');
      }}
    >
      <Button
        type="text"
        icon={<SearchOutlined />}
        aria-label={intl.formatMessage({ id: 'navigation.search.open' })}
      />
    </Popover>
  );
}
