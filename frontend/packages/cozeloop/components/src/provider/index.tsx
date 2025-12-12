// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createContext, useContext, type ReactNode } from 'react';

// I18n function type definition
export type I18nFunction = (
  key: string,
  options?: Record<string, unknown>,
  fallbackText?: string,
) => string;

// I18n object interface that matches the structure of @cozeloop/i18n-adapter
export interface I18nObject {
  t: I18nFunction;
  language?: string; // en-US,zh-CN
}

// Context interface
interface CozeLoopContextValue {
  i18n?: I18nFunction | I18nObject;
  sendEvent?: (
    name: string,
    params: Record<string, unknown>,
    target?: HTMLElement | string | null,
  ) => void;
}

// Create context
const CozeLoopContext = createContext<CozeLoopContextValue>({});

// Provider props
interface CozeLoopProviderProps extends CozeLoopContextValue {
  children: ReactNode;
}

// Provider component
export function CozeLoopProvider({
  children,
  i18n,
  sendEvent,
}: CozeLoopProviderProps) {
  return (
    <CozeLoopContext.Provider value={{ i18n, sendEvent }}>
      {children}
    </CozeLoopContext.Provider>
  );
}

// Hook to use i18n - returns an object with t method to maintain I18n.t usage pattern
export function useI18n(): I18nObject {
  const context = useContext(CozeLoopContext);

  if (!context.i18n) {
    // Fallback object when no i18n is provided
    return {
      t: (key: string) => {
        console.warn(
          `CozeLoopProvider: No i18n function provided, returning key: ${key}`,
        );
        return key;
      },
      language: 'zh-CN',
    };
  }

  // If context.i18n is already an object with t method, return it directly
  if (typeof context.i18n === 'object' && 't' in context.i18n) {
    return context.i18n;
  }

  // If context.i18n is a function, wrap it in an object
  return {
    t: context.i18n,
  };
}

export type I18nType = ReturnType<typeof useI18n>;

// Hook to use sendEvent - return sendEvent function
export function useReportEvent() {
  const context = useContext(CozeLoopContext);
  return (
    context.sendEvent ??
    ((name: string, params: Record<string, unknown>) => {
      console.info(name, params);
    })
  );
}

// Hook to get the full context (for advanced usage)
export function useCozeLoopContext(): CozeLoopContextValue {
  return useContext(CozeLoopContext);
}
