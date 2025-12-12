import type React from 'react';
import { useMemo, useRef } from 'react';

import qs from 'query-string';
import type { ParseOptions, StringifyOptions } from 'query-string';
import { useMemoizedFn } from 'ahooks';

import { useUrlStateStore } from '@/shared/stores';

export interface Options {
  navigateMode?: 'push' | 'replace';
  parseOptions?: ParseOptions;
  stringifyOptions?: StringifyOptions;
}

const baseParseConfig: ParseOptions = {
  parseNumbers: false,
  parseBooleans: false,
  arrayFormat: 'bracket',
};

type UrlState = Record<string, unknown>;

export const useUrlState = <S extends UrlState = UrlState>(
  initialState?: S | (() => S),
  options?: Options,
) => {
  type State = S;
  const {
    navigateMode = 'replace',
    parseOptions,
    stringifyOptions,
  } = options || {};

  const search = useUrlStateStore(state => state.search);
  const updateUrl = useUrlStateStore(state => state.updateUrl);

  const initialStateRef = useRef(
    typeof initialState === 'function'
      ? (initialState as () => S)()
      : initialState || {},
  );

  const queryFromUrl = useMemo(() => {
    const mergedParseOptions = { ...baseParseConfig, ...parseOptions };
    return qs.parse(search, mergedParseOptions);
  }, [search, parseOptions]);

  const targetQuery = useMemo(
    () => ({
      ...initialStateRef.current,
      ...queryFromUrl,
    }),
    [queryFromUrl],
  ) as State;

  const setState = (s: React.SetStateAction<State>) => {
    const newQuery = typeof s === 'function' ? s(targetQuery) : s;

    updateUrl(
      newQuery,
      {
        navigateMode,
        parseOptions,
        stringifyOptions,
      },
      initialStateRef.current,
    );
  };

  return [targetQuery, useMemoizedFn(setState)] as [
    State,
    (s: React.SetStateAction<State>) => void,
  ];
};
