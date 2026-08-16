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

export function useManagementRouteIntent<T>(
  intent: ManagementRouteIntent | undefined,
  handlers: ManagementRouteHandlers<T>,
): void {
  const handlersRef = useRef(handlers);
  const handledKeyRef = useRef<string | undefined>(undefined);
  handlersRef.current = handlers;
  const action = intent?.action;
  const id = intent && 'id' in intent ? intent.id : undefined;

  useEffect(() => {
    if (!action) {
      handledKeyRef.current = undefined;
      return;
    }
    const key = id ? `${action}:${id}` : action;
    if (handledKeyRef.current === key) return;
    handledKeyRef.current = key;

    const current = handlersRef.current;
    if (action === 'create') {
      current.openCreate();
      return;
    }
    if (!id) return;

    let active = true;
    void current
      .load(id)
      .then((entity) => {
        if (!active) return;
        if (action === 'reset-password') {
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
  }, [action, id]);
}

export function finishManagementRouteIntent(
  intent: ManagementRouteIntent | undefined,
  listPath: string,
): void {
  if (intent) history.replace(listPath);
}
