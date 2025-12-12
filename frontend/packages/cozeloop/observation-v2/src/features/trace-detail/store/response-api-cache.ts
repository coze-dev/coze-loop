import { immer } from 'zustand/middleware/immer';
import { create } from 'zustand';
import { LRUCache } from 'lru-cache';
import { type span } from '@cozeloop/api-schema/observation';

export interface ResponseCacheItem {
  data: span.OutputSpan[] | null;
  isFetched: boolean;
}

export interface ResponseCacheState {
  cache: LRUCache<string, ResponseCacheItem>;
}

interface ResponseCacheAction {
  set: (key: string, value: ResponseCacheItem) => void;
  delete: (key: string) => boolean;
  clear: () => void;
}

export const useResponseApiCacheStore = create<
  ResponseCacheState & ResponseCacheAction
>()(
  immer(set => ({
    cache: new LRUCache<string, ResponseCacheItem>({
      max: 200,
    }),
    set: (key, value) => {
      set(state => {
        state.cache.set(key, value);
      });
    },
    delete: key => {
      let result = false;
      set(state => {
        result = state.cache.delete(key);
      });
      return result;
    },
    clear: () => {
      set(state => {
        state.cache.clear();
      });
    },
  })),
);
