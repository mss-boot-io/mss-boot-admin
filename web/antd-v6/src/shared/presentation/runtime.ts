import { useQuery } from '@tanstack/react-query';
import { request } from '@umijs/max';
import { hasPermission } from '../auth/access';
import type { CurrentUser } from '../auth/types';
import {
  type AdminLocale,
  buildPageRenderModel,
  type PageCapabilityDefinition,
  type PageRenderModel,
  resolvePagePresentation,
} from './contract';
import {
  type EffectivePresentationResponse,
  parseEffectivePresentationResponse,
} from './effective';

export interface PresentationRuntimeDiagnostic {
  code: string;
  layer?: 'application' | 'role' | 'user';
}

export interface PagePresentationRuntime {
  definition: PageCapabilityDefinition;
  model: PageRenderModel;
  shadowModel?: PageRenderModel;
  source: 'compiled' | 'active';
  settled: boolean;
  diagnostics: readonly PresentationRuntimeDiagnostic[];
}

export interface EffectivePresentationRequestOptions {
  method: 'GET';
  signal: AbortSignal;
  skipErrorHandler: true;
}

export type EffectivePresentationRequestClient = (
  path: string,
  options: EffectivePresentationRequestOptions,
) => Promise<unknown>;

export interface PresentationRegistryEntry {
  definitionHash: string;
  definition: PageCapabilityDefinition;
}

function grantedPermissions(
  definition: PageCapabilityDefinition,
  user: CurrentUser | undefined,
): ReadonlySet<string> {
  const required = new Set<string>();
  for (const dataSource of definition.dataSources) {
    for (const permission of dataSource.requiredPermissions) required.add(permission);
  }
  for (const action of definition.actions) {
    for (const permission of action.requiredPermissions) required.add(permission);
  }
  return new Set([...required].filter((permission) => hasPermission(user, permission)));
}

export function resolveEffectivePagePresentation(input: {
  entry: PresentationRegistryEntry;
  locale: AdminLocale;
  user?: CurrentUser;
  response?: EffectivePresentationResponse;
  settled: boolean;
  failureCode?: string;
}): PagePresentationRuntime {
  const permissions = grantedPermissions(input.entry.definition, input.user);
  const compiledResolution = resolvePagePresentation(input.entry.definition, {}, permissions);
  const compiledModel = buildPageRenderModel(
    input.entry.definition,
    compiledResolution,
    input.locale,
  );
  const diagnostics: PresentationRuntimeDiagnostic[] = input.response
    ? input.response.diagnostics.map(({ code, layer }) => ({ code, ...(layer ? { layer } : {}) }))
    : input.failureCode
      ? [{ code: input.failureCode }]
      : [];
  const response = input.response;
  if (
    !response ||
    response.pageKey !== input.entry.definition.pageKey ||
    response.definitionHash !== input.entry.definitionHash ||
    response.definitionHash !== input.entry.definition.definitionHash
  ) {
    return {
      definition: input.entry.definition,
      model: compiledModel,
      source: 'compiled',
      settled: input.settled,
      diagnostics,
    };
  }

  let resolved: PageRenderModel;
  try {
    const resolution = resolvePagePresentation(
      input.entry.definition,
      response.layers,
      permissions,
    );
    for (const rejected of resolution.rejectedLayers) {
      diagnostics.push({ code: 'runtime-layer-rejected', layer: rejected.layer });
    }
    resolved = buildPageRenderModel(input.entry.definition, resolution, input.locale);
  } catch {
    return {
      definition: input.entry.definition,
      model: compiledModel,
      source: 'compiled',
      settled: input.settled,
      diagnostics: [...diagnostics, { code: 'runtime-resolution-rejected' }],
    };
  }
  if (!response.adoption.applyLayers) {
    return {
      definition: input.entry.definition,
      model: compiledModel,
      ...(response.adoption.resolveLayers ? { shadowModel: resolved } : {}),
      source: 'compiled',
      settled: input.settled,
      diagnostics,
    };
  }
  return {
    definition: input.entry.definition,
    model: resolved,
    source: 'active',
    settled: input.settled,
    diagnostics,
  };
}

export {
  PresentationFieldControl,
  type PresentationSelectOption,
  renderPresentationValue,
} from './components';
export { buildPresentationConditionContext, evaluatePresentationCondition } from './condition';
export type {
  EffectivePresentationAdoption,
  EffectivePresentationDiagnostic,
  EffectivePresentationResponse,
  PresentationAdoptionMode,
  PresentationAdoptionState,
} from './effective';

export function createEffectivePresentationAPI(client: EffectivePresentationRequestClient) {
  return {
    load: async (pageKey: string, signal: AbortSignal, timeoutMs = 1_500) => {
      const controller = new AbortController();
      const abort = () => controller.abort();
      signal.addEventListener('abort', abort, { once: true });
      const timer = globalThis.setTimeout(abort, timeoutMs);
      try {
        return parseEffectivePresentationResponse(
          await client(`/presentation/effective/${encodeURIComponent(pageKey)}`, {
            method: 'GET',
            signal: controller.signal,
            skipErrorHandler: true,
          }),
        );
      } finally {
        globalThis.clearTimeout(timer);
        signal.removeEventListener('abort', abort);
      }
    },
  };
}

const effectivePresentationAPI = createEffectivePresentationAPI((path, options) =>
  request<unknown>(path, options),
);

export function usePagePresentation(
  entry: PresentationRegistryEntry,
  locale: AdminLocale,
  user: CurrentUser | undefined,
  authorizationVersion = 0,
): PagePresentationRuntime {
  const pageKey = entry.definition.pageKey;
  const effective = useQuery({
    queryKey: [
      'presentation',
      'effective',
      pageKey,
      entry.definitionHash,
      user?.id ?? '',
      authorizationVersion,
    ],
    queryFn: ({ signal }) => effectivePresentationAPI.load(pageKey, signal),
    enabled: Boolean(user),
    retry: false,
    staleTime: 15_000,
    refetchOnWindowFocus: false,
  });
  return resolveEffectivePagePresentation({
    entry,
    locale,
    user,
    response: effective.data,
    settled: !effective.isPending,
    ...(effective.isError ? { failureCode: 'effective-read-failed' } : {}),
  });
}
