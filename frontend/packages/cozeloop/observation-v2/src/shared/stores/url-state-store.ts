import { devtools } from 'zustand/middleware';
import { create } from 'zustand';
import qs from 'query-string';
import type { ParseOptions, StringifyOptions } from 'query-string';

export interface UrlStateOptions {
  navigateMode?: 'push' | 'replace';
  parseOptions?: ParseOptions;
  stringifyOptions?: StringifyOptions;
}

type UrlState = Record<string, unknown>;

interface UrlStateStore {
  search: string;
  updateUrl: <S extends UrlState>(
    state: S,
    options?: UrlStateOptions,
    initialState?: S,
  ) => void;
}

const baseParseConfig: ParseOptions = {
  parseNumbers: false,
  parseBooleans: false,
  arrayFormat: 'bracket',
};

const baseStringifyConfig: StringifyOptions = {
  skipNull: false,
  skipEmptyString: false,
  arrayFormat: 'bracket',
};

export const useUrlStateStore = create<UrlStateStore>()(
  devtools(
    (set, get) => ({
      search: typeof window !== 'undefined' ? window.location.search : '',

      updateUrl: <S extends UrlState>(
        newState: S,
        options?: UrlStateOptions,
        initialState?: S,
      ) => {
        const {
          navigateMode = 'replace',
          parseOptions,
          stringifyOptions,
        } = options || {};

        const mergedParseOptions = { ...baseParseConfig, ...parseOptions };
        const mergedStringifyOptions = {
          ...baseStringifyConfig,
          ...stringifyOptions,
        };

        const currentQuery = qs.parse(get().search, mergedParseOptions);
        const mergedQuery = {
          ...(initialState || {}),
          ...currentQuery,
          ...newState,
        };

        const queryString = qs.stringify(mergedQuery, mergedStringifyOptions);
        const search = queryString ? `?${queryString}` : '';

        const newUrl = `${window.location.pathname}${search}${window.location.hash}`;

        if (navigateMode === 'replace') {
          window.history.replaceState(window.history.state, '', newUrl);
        } else {
          window.history.pushState(window.history.state, '', newUrl);
        }

        set({ search });
      },
    }),
    {
      name: 'url-state',
      enabled: process.env.NODE_ENV === 'development',
    },
  ),
);

if (typeof window !== 'undefined') {
  const handlePopstate = () => {
    useUrlStateStore.setState({ search: window.location.search });
  };

  window.addEventListener('popstate', handlePopstate);
}
