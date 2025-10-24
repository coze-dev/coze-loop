/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
