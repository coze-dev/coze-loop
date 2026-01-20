// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-empty-interface */
import {
  createContext,
  type PropsWithChildren,
  useContext,
  useEffect,
  useRef,
} from 'react';

import { createStore } from 'zustand/vanilla';
import { useStore, type StoreApi } from 'zustand';
import { type LRUCache } from 'lru-cache';

import { type ResponseCacheItem } from '@/features/trace-detail/store/response-api-cache';

export interface ResponseApiCacheState {
  responseApiCache?: LRUCache<string, ResponseCacheItem, unknown>;
}

type ResponseApiCacheStore = StoreApi<ResponseApiCacheState>;

const createResponseApiCacheStore = (
  initState: ResponseApiCacheState,
): ResponseApiCacheStore => createStore(() => initState);

const ResponseApiCacheContext = createContext<ResponseApiCacheStore | null>(
  null,
);

const fallbackStore = createResponseApiCacheStore({
  responseApiCache: undefined,
});

export interface ResponseApiCacheProviderProps
  extends PropsWithChildren<ResponseApiCacheState> {}

export const ResponseApiCacheProvider = ({
  children,
  responseApiCache,
}: ResponseApiCacheProviderProps) => {
  const storeRef = useRef<ResponseApiCacheStore>();

  if (!storeRef.current) {
    storeRef.current = createResponseApiCacheStore({
      responseApiCache,
    });
  }

  useEffect(() => {
    storeRef.current?.setState({ responseApiCache });
  }, [responseApiCache]);

  return (
    <ResponseApiCacheContext.Provider value={storeRef.current}>
      {children}
    </ResponseApiCacheContext.Provider>
  );
};

export const useResponseApiCacheStore = <T,>(
  selector: (state: ResponseApiCacheState) => T,
): T => {
  const store = useContext(ResponseApiCacheContext) ?? fallbackStore;
  return useStore(store, selector);
};
