import { useCallback, useRef, useState } from 'react';

/**
 * Serializes task operations before React has had a chance to render the
 * pending state. The ref is the synchronous guard; state only drives the UI.
 */
export const useTaskOperationGuard = () => {
  const pendingTaskIDRef = useRef<string>();
  const [pendingTaskID, setPendingTaskID] = useState<string>();

  const runTaskOperation = useCallback(
    async (taskID: string | undefined, operation: () => Promise<void>): Promise<boolean> => {
      if (!taskID || pendingTaskIDRef.current) {
        return false;
      }

      pendingTaskIDRef.current = taskID;
      setPendingTaskID(taskID);
      try {
        await operation();
        return true;
      } finally {
        pendingTaskIDRef.current = undefined;
        setPendingTaskID(undefined);
      }
    },
    [],
  );

  return { pendingTaskID, runTaskOperation };
};
