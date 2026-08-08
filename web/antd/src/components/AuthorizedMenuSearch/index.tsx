import HeaderSearch from '@/components/HeaderSearch';
import { getMenuLocaleId } from '@/pages/Menu/menuLocale';
import {
  requestAuthorizedMenu,
  type AuthorizedMenuNode,
} from '@/utils/requestAuthorizedMenu';
import type { AuthorizationIdentity } from '@/utils/authorization';
import { history, useIntl } from '@umijs/max';
import { Button, Space, Spin, Typography } from 'antd';
import React, { useEffect, useMemo, useState } from 'react';

const SEARCH_RESULT_LIMIT = 20;

const MENU_PATH_LOCALE_IDS: Record<string, string> = {
  '/workplace': 'menu.welcome',
  '/origination': 'menu.origination',
  '/departments': 'menu.origination.department',
  '/posts': 'menu.origination.post',
  '/users': 'menu.origination.user',
  '/authority': 'menu.authority',
  '/role': 'menu.authority.role',
  '/menu': 'menu.authority.menu',
  '/system': 'menu.system',
  '/task': 'menu.system.task',
  '/language': 'menu.system.language',
  '/notice': 'menu.system.notice',
  '/option': 'menu.system.option',
  '/super-permission': 'menu.super-permission',
  '/app-config': 'menu.super-permission.appConfig',
  '/system-config': 'menu.super-permission.system-config',
  '/log': 'menu.system.log',
  '/security': 'menu.security',
  '/security/online-sessions': 'menu.security.online-sessions',
};

export type AuthorizedMenuSearchItem = {
  label: string;
  path: string;
  searchText: string;
};

export type AuthorizedMenuSearchProps = {
  identity?: AuthorizationIdentity;
  permissionRefreshVersion?: number;
};

const normalizeSearchText = (value: string) => value.normalize('NFKC').trim().toLocaleLowerCase();

const isNavigableMenuPath = (path?: string) =>
  Boolean(path?.startsWith('/') && !path.startsWith('//') && !/[:*]/.test(path));

export const buildAuthorizedMenuSearchItems = (
  menu: AuthorizedMenuNode[],
  localizeName: (name: string, path?: string) => string,
): AuthorizedMenuSearchItem[] => {
  const items: AuthorizedMenuSearchItem[] = [];
  const seenPaths = new Set<string>();

  const visit = (nodes: AuthorizedMenuNode[], parentLabels: string[]) => {
    nodes.forEach((node) => {
      if (node.hideInMenu || node.status === 'disabled') return;

      const rawName = node.name?.trim() || '';
      const localizedName = localizeName(rawName, node.path);
      const currentLabel = localizedName || node.path || '';
      const labels = currentLabel ? [...parentLabels, currentLabel] : parentLabels;
      const navigableType = !node.type || node.type === 'MENU';

      if (
        navigableType &&
        isNavigableMenuPath(node.path) &&
        node.path &&
        !seenPaths.has(node.path)
      ) {
        seenPaths.add(node.path);
        const label = labels.join(' / ') || node.path;
        items.push({
          label,
          path: node.path,
          searchText: normalizeSearchText(
            [label, localizedName, rawName, node.path].filter(Boolean).join(' '),
          ),
        });
      }

      if (!node.hideChildrenInMenu && node.children?.length) {
        visit(node.children, labels);
      }
    });
  };

  visit(menu, []);
  return items;
};

export const filterAuthorizedMenuSearchItems = (
  items: AuthorizedMenuSearchItem[],
  query: string,
) => {
  const normalizedQuery = normalizeSearchText(query);
  if (!normalizedQuery) return [];
  return items
    .filter((item) => item.searchText.includes(normalizedQuery))
    .slice(0, SEARCH_RESULT_LIMIT);
};

const AuthorizedMenuSearch: React.FC<AuthorizedMenuSearchProps> = ({
  identity,
  permissionRefreshVersion = 0,
}) => {
  const intl = useIntl();
  const [menu, setMenu] = useState<AuthorizedMenuNode[]>([]);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error>();
  const [retryVersion, setRetryVersion] = useState(0);

  useEffect(() => {
    let active = true;
    setMenu([]);
    setLoading(true);
    setError(undefined);

    requestAuthorizedMenu(identity, permissionRefreshVersion)
      .then((nextMenu) => {
        if (active) setMenu(nextMenu);
      })
      .catch((requestError) => {
        if (!active) return;
        setError(
          requestError instanceof Error
            ? requestError
            : new Error('Unable to load the authorized menu'),
        );
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => {
      active = false;
    };
  }, [identity, permissionRefreshVersion, retryVersion]);

  const items = useMemo(
    () =>
      buildAuthorizedMenuSearchItems(menu, (name, path) => {
        const normalizedPath = path?.split(/[?#]/, 1)[0].replace(/\/+$/, '') || '';
        const nameIsPath = name.startsWith('/');
        const nameID = name && !nameIsPath ? getMenuLocaleId(name) : undefined;
        const pathID = MENU_PATH_LOCALE_IDS[normalizedPath];
        const hasNameMessage =
          !!nameID && Object.prototype.hasOwnProperty.call(intl.messages || {}, nameID);
        const id = hasNameMessage ? nameID : pathID || nameID;
        if (!id) return name;
        return intl.formatMessage({ id, defaultMessage: (nameIsPath ? path : name) || path || id });
      }),
    [intl, menu],
  );
  const results = useMemo(() => filterAuthorizedMenuSearchItems(items, query), [items, query]);
  const options = useMemo(
    () =>
      results.map((item) => ({
        value: item.path,
        label: (
          <Space direction="vertical" size={0}>
            <Typography.Text>{item.label}</Typography.Text>
            <Typography.Text type="secondary">{item.path}</Typography.Text>
          </Space>
        ),
      })),
    [results],
  );

  const notFoundContent = loading ? (
    <Space role="status">
      <Spin size="small" />
      {intl.formatMessage({
        id: 'component.search.loading',
        defaultMessage: 'Loading menus…',
      })}
    </Space>
  ) : error ? (
    <Space role="alert">
      {intl.formatMessage({
        id: 'component.search.loadFailed',
        defaultMessage: 'Unable to load menus.',
      })}
      <Button
        type="link"
        size="small"
        onMouseDown={(event) => event.preventDefault()}
        onClick={() => setRetryVersion((version) => version + 1)}
      >
        {intl.formatMessage({ id: 'component.search.retry', defaultMessage: 'Retry' })}
      </Button>
    </Space>
  ) : (
    <span role="status">
      {intl.formatMessage({
        id: 'component.search.noResults',
        defaultMessage: 'No accessible menus found.',
      })}
    </span>
  );

  return (
    <HeaderSearch
      value={query}
      onChange={(value) => setQuery(value || '')}
      onSelect={(value) => {
        setQuery('');
        history.push(String(value));
      }}
      options={options}
      notFoundContent={notFoundContent}
      placeholder="component.search.placeholder"
      triggerLabel="component.search.open"
    />
  );
};

export default AuthorizedMenuSearch;
