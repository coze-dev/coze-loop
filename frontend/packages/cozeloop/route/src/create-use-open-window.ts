// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback } from 'react';

import { getPath } from './utils';
import {
  type RouteInfoURLParams,
  type UseRouteInfo,
  type UseOpenWindow,
} from './types';

/**
 * 处理链接跳转
 * @returns
 */
export const createUseOpenWindow =
  (useRouteInfo: UseRouteInfo): UseOpenWindow =>
  () => {
    const { getBaseURL } = useRouteInfo();

    const getURL = useCallback(
      (path: string, params?: RouteInfoURLParams) => {
        if (path.startsWith('http://') || path.startsWith('https://')) {
          return path;
        }
        const dynamicBaseURL = getBaseURL(params);
        return getPath(path, dynamicBaseURL);
      },
      [getBaseURL],
    );
    /**
     * 打开新窗口
     */
    const openBlank = useCallback(
      (path: string, params?: RouteInfoURLParams) => {
        window.open(getURL(path, params));
      },
      [getURL],
    );

    /**
     * 原窗口加载地址
     */
    const openSelf = useCallback(
      (path: string, params?: RouteInfoURLParams) => {
        window.open(getURL(path, params), '_self');
      },
      [getURL],
    );

    return {
      openBlank,
      openSelf,
      getURL,
    };
  };
