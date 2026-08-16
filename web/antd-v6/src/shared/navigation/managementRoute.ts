import { history } from '@umijs/max';
import { useEffect, useRef } from 'react';

export type ManagementRouteIntent =
  | { action: 'create' }
  | { action: 'edit'; id: string }
  | { action: 'reset-password'; id: string };

interface ManagementRouteHandlers<T> {
  load: (id: string) => Promise<T>;
  onError: (error: unknown) => void;
  openCreate: () => void;
  openEdit: (entity: T) => void;
  openResetPassword?: (entity: T) => void;
}

function intentKey(intent: ManagementRouteIntent): string {
  return intent.action === 'create' ? intent.action : `${intent.action}:${intent.id}`;
}

export function useManagementRouteIntent<T>(
  intent: ManagementRouteIntent | undefined,
  handlers: ManagementRouteHandlers<T>,
): void {
  const handlersRef = useRef(handlers);
  const handledKeyRef = useRef<string | undefined>(undefined);
  handlersRef.current = handlers;

  useEffect(() => {
    if (!intent) return;
    const key = intentKey(intent);
    if (handledKeyRef.current === key) return;
    handledKeyRef.current = key;

    const current = handlersRef.current;
    if (intent.action === 'create') {
      current.openCreate();
      return;
    }

    let active = true;
    void current
      .load(intent.id)
      .then((entity) => {
        if (!active) return;
        if (intent.action === 'reset-password') {
          current.openResetPassword?.(entity);
        } else {
          current.openEdit(entity);
        }
      })
      .catch((error: unknown) => {
        if (active) current.onError(error);
      });

    return () => {
      active = false;
    };
  }, [intent?.action, intent && 'id' in intent ? intent.id : undefined]);
}

export function finishManagementRouteIntent(
  intent: ManagementRouteIntent | undefined,
  listPath: string,
): void {
  if (intent) history.replace(listPath);
}
