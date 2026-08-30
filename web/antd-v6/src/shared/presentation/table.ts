import type { TableColumnsType } from 'antd';
import {
  type Dispatch,
  type SetStateAction,
  useCallback,
  useEffect,
  useRef,
  useState,
} from 'react';
import type { PageRenderField, PageRenderModel, PresentationDensity } from './contract';

type TableColumn<TRecord> = TableColumnsType<TRecord>[number];
type ComponentBindings = Readonly<Record<string, string>>;

export interface TablePresentationResult<
  TRecord,
  TSearchField extends string,
  TPageSize extends number,
> {
  columns: TableColumnsType<TRecord>;
  density: PresentationDensity;
  mobileColumnKeys: readonly string[];
  pageSize: TPageSize;
  searchCollapsedByDefault: boolean;
  searchFields: ReadonlyMap<TSearchField, PageRenderField>;
}

interface TablePresentationInput<
  TRecord,
  TListComponents extends ComponentBindings,
  TSearchComponents extends ComponentBindings,
  TPageSize extends number,
> {
  compiledColumns: TableColumnsType<TRecord>;
  fallbackPageSize: TPageSize;
  isPageSize: (value: number) => value is TPageSize;
  listComponents: TListComponents;
  mobileColumnKeys?: readonly string[];
  model: PageRenderModel;
  protectedColumnKeys?: readonly string[];
  searchComponents: TSearchComponents;
}

function tableColumnKey<TRecord>(column: TableColumn<TRecord>): string | undefined {
  if ('dataIndex' in column && typeof column.dataIndex === 'string') return column.dataIndex;
  if ('key' in column && typeof column.key === 'string') return column.key;
  return undefined;
}

/**
 * Applies presentation-only list and search facts to compiled table controls.
 *
 * Runtime fields are fail-closed against exact field/component bindings. Protected
 * columns (for example mutation and authorization actions) are appended from the
 * compiled table unchanged and can never be created, hidden, relabelled, reordered,
 * or resized by a profile.
 */
export function resolveTablePresentation<
  TRecord,
  TListComponents extends ComponentBindings,
  TSearchComponents extends ComponentBindings,
  TPageSize extends number,
>(
  input: TablePresentationInput<TRecord, TListComponents, TSearchComponents, TPageSize>,
): TablePresentationResult<TRecord, Extract<keyof TSearchComponents, string>, TPageSize> {
  const compiledByKey = new Map<string, TableColumn<TRecord>>();
  for (const column of input.compiledColumns) {
    const key = tableColumnKey(column);
    if (key) compiledByKey.set(key, column);
  }

  const protectedKeys = new Set(input.protectedColumnKeys ?? []);
  const mobileKeys = new Set(input.mobileColumnKeys ?? []);
  const resolvedMobileKeys: string[] = [];
  const resolvedBusinessKeys: string[] = [];
  const columns: TableColumnsType<TRecord> = [];
  for (const field of input.model.list.columns) {
    if (protectedKeys.has(field.field)) continue;
    const expectedComponent = input.listComponents[field.field];
    const compiled = compiledByKey.get(field.field);
    if (!compiled || !expectedComponent || field.component !== expectedComponent) continue;
    resolvedBusinessKeys.push(field.field);
    if (mobileKeys.has(field.field)) resolvedMobileKeys.push(field.field);
    columns.push({
      ...compiled,
      title: field.label,
      ...(field.width !== undefined ? { width: field.width } : {}),
    });
  }

  for (const key of input.protectedColumnKeys ?? []) {
    const column = compiledByKey.get(key);
    if (!column) continue;
    columns.push(column);
    if (mobileKeys.has(key)) resolvedMobileKeys.push(key);
  }

  // Static mobile preferences are compiled for the default presentation. If a
  // valid runtime profile hides every preferred business column, keep the first
  // configured visible business column so mobile never degrades to actions only.
  if (
    resolvedBusinessKeys.length > 0 &&
    !resolvedMobileKeys.some((key) => !protectedKeys.has(key))
  ) {
    const fallbackKey = resolvedBusinessKeys[0];
    if (fallbackKey !== undefined) resolvedMobileKeys.unshift(fallbackKey);
  }

  type SearchField = Extract<keyof TSearchComponents, string>;
  const searchFields = new Map<SearchField, PageRenderField>();
  for (const field of input.model.search.fields) {
    const expectedComponent = input.searchComponents[field.field];
    if (!expectedComponent || field.component !== expectedComponent) continue;
    searchFields.set(field.field as SearchField, field);
  }

  return {
    columns,
    density: input.model.list.density,
    mobileColumnKeys: resolvedMobileKeys,
    pageSize: input.isPageSize(input.model.list.pageSize)
      ? input.model.list.pageSize
      : input.fallbackPageSize,
    searchCollapsedByDefault: input.model.search.collapsedByDefault,
    searchFields,
  };
}

/**
 * Tracks a presentation page-size update until the operator changes table state.
 * This preserves the compiled first render while allowing an asynchronously loaded
 * active profile to establish the initial page size.
 */
export function usePresentationPageParams<TParams extends { current: number; pageSize: number }>(
  initialParams: TParams,
  presentationPageSize: number,
): readonly [TParams, Dispatch<SetStateAction<TParams>>] {
  const [params, setParams] = useState<TParams>(() => ({
    ...initialParams,
    pageSize: presentationPageSize,
  }));
  const queryWasChanged = useRef(false);
  const appliedPresentationPageSize = useRef(presentationPageSize);
  const updateParams = useCallback<Dispatch<SetStateAction<TParams>>>((value) => {
    queryWasChanged.current = true;
    setParams(value);
  }, []);

  useEffect(() => {
    if (queryWasChanged.current || appliedPresentationPageSize.current === presentationPageSize) {
      return;
    }
    setParams((current) => ({
      ...current,
      current: 1,
      pageSize: presentationPageSize,
    }));
    appliedPresentationPageSize.current = presentationPageSize;
  }, [presentationPageSize]);

  return [params, updateParams] as const;
}

/** Keeps the compiled search form usable while honoring a profile's initial collapse state. */
export function usePresentationSearchExpansion(searchCollapsedByDefault: boolean) {
  const [expanded, setExpanded] = useState(() => !searchCollapsedByDefault);
  const expansionWasChanged = useRef(false);

  useEffect(() => {
    if (!expansionWasChanged.current) setExpanded(!searchCollapsedByDefault);
  }, [searchCollapsedByDefault]);

  const expand = useCallback(() => {
    expansionWasChanged.current = true;
    setExpanded(true);
  }, []);

  return { expand, expanded } as const;
}
