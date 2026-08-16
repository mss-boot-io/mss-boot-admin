import { createContext, type ReactNode, useContext } from 'react';

export type RuntimeMessageFormatter = (messageID: string) => string;

const RuntimeMessageContext = createContext<RuntimeMessageFormatter>((messageID) => messageID);

export function RuntimeMessageProvider({
  children,
  formatMessage,
}: {
  children: ReactNode;
  formatMessage: RuntimeMessageFormatter;
}) {
  return (
    <RuntimeMessageContext.Provider value={formatMessage}>
      {children}
    </RuntimeMessageContext.Provider>
  );
}

export function useRuntimeMessage(): RuntimeMessageFormatter {
  return useContext(RuntimeMessageContext);
}
