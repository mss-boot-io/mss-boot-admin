import type { QueryKey } from '@tanstack/react-query';
import type { AuthorizedMenuItem, CurrentUser } from './types';

export const AUTHORIZATION_REFRESH_EVENT = 'mss:authorization-refresh';
export const AUTHORIZATION_REFRESH_CHANNEL = 'mss-authorization-v1';
export const AUTHORIZATION_REFRESH_THROTTLE_MS = 30_000;

interface AuthorizationRefreshSignal {
  v: 1;
  kind: 'authorization-refresh';
  source: string;
  sentAt: number;
  revision?: string;
}

const sourceID = (() => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `runtime-${Date.now()}-${Math.random().toString(36).slice(2)}`;
})();

let highestRealtimeRevision: string | undefined;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function shouldRefreshAuthorization(
  lastRefreshAt: number,
  now: number,
  throttleMs = AUTHORIZATION_REFRESH_THROTTLE_MS,
): boolean {
  return now - lastRefreshAt >= throttleMs;
}

export function isCanonicalAuthorizationRevision(value: unknown): value is string {
  return typeof value === 'string' && /^[1-9]\d*$/.test(value);
}

export function isNewerAuthorizationRevision(candidate: string, current?: string): boolean {
  if (!isCanonicalAuthorizationRevision(candidate)) return false;
  if (!current) return true;
  if (!isCanonicalAuthorizationRevision(current)) return true;
  return (
    candidate.length > current.length ||
    (candidate.length === current.length && candidate > current)
  );
}

function menuSignature(items: readonly AuthorizedMenuItem[]): unknown[] {
  return items.map((item) => ({
    key: item.key,
    path: item.path,
    permission: item.permission,
    rootOnly: item.rootOnly === true,
    children: menuSignature(item.children ?? []),
  }));
}

/**
 * Only privilege-bearing identity fields and executable menu metadata enter
 * this signature. Profile edits do not evict unrelated server-state queries.
 */
export function authorizationStateSignature(
  user: CurrentUser | undefined,
  menu: readonly AuthorizedMenuItem[],
): string {
  return JSON.stringify({
    user: user
      ? {
          id: user.id,
          roleID: user.roleID,
          role: user.role
            ? {
                id: user.role.id,
                root: user.role.root === true,
                status: user.role.status,
              }
            : undefined,
          permissions: Object.entries(user.permissions).sort(([left], [right]) =>
            left.localeCompare(right),
          ),
        }
      : undefined,
    menu: menuSignature(menu),
  });
}

/** Queries required to establish identity and render the application shell. */
export function isAuthorizationBootstrapQuery(queryKey: QueryKey): boolean {
  const [domain, resource] = queryKey;
  return (
    (domain === 'identity' && resource === 'current-user') ||
    (domain === 'authorization' && resource === 'menu') ||
    (domain === 'configuration' && (resource === 'application-profile' || resource === 'theme'))
  );
}

function parseRefreshSignal(value: unknown): AuthorizationRefreshSignal | undefined {
  if (!isRecord(value)) return undefined;
  if (
    value.v !== 1 ||
    value.kind !== 'authorization-refresh' ||
    typeof value.source !== 'string' ||
    !value.source ||
    typeof value.sentAt !== 'number' ||
    !Number.isFinite(value.sentAt) ||
    (value.revision !== undefined && !isCanonicalAuthorizationRevision(value.revision))
  ) {
    return undefined;
  }
  return value as unknown as AuthorizationRefreshSignal;
}

function emitAuthorizationRefresh(revision?: string): void {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(AUTHORIZATION_REFRESH_EVENT));
  }
  if (typeof BroadcastChannel === 'undefined') return;
  try {
    const channel = new BroadcastChannel(AUTHORIZATION_REFRESH_CHANNEL);
    channel.postMessage({
      v: 1,
      kind: 'authorization-refresh',
      source: sourceID,
      sentAt: Date.now(),
      revision,
    } satisfies AuthorizationRefreshSignal);
    channel.close();
  } catch {
    // Focus/visibility reconciliation remains the compatibility fallback.
  }
}

export function requestAuthorizationRefresh(): void {
  emitAuthorizationRefresh();
}

/**
 * Deduplicates one global server revision across this tab's socket and
 * BroadcastChannel. A disconnected sibling tab can still receive the signal.
 */
export function requestAuthorizationRevisionRefresh(revision: string): void {
  if (!isNewerAuthorizationRevision(revision, highestRealtimeRevision)) return;
  highestRealtimeRevision = revision;
  emitAuthorizationRefresh(revision);
}

export function subscribeAuthorizationRefreshBroadcast(onRefresh: () => void): () => void {
  if (typeof BroadcastChannel === 'undefined') return () => {};
  try {
    const channel = new BroadcastChannel(AUTHORIZATION_REFRESH_CHANNEL);
    channel.onmessage = (event) => {
      const signal = parseRefreshSignal(event.data);
      if (!signal || signal.source === sourceID) return;
      if (signal.revision) {
        if (!isNewerAuthorizationRevision(signal.revision, highestRealtimeRevision)) return;
        highestRealtimeRevision = signal.revision;
      }
      onRefresh();
    };
    return () => channel.close();
  } catch {
    return () => {};
  }
}
