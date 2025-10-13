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

import {
  useNavigate as useBaseNavigate,
  type NavigateOptions,
  type To,
} from 'react-router-dom';
import { useCallback } from 'react';

import { getPath } from './utils';
import {
  type RouteInfoURLParams,
  type UseRouteInfo,
  type UseNavigateModule,
} from './types';

/**
 * 基于模块的 navigate，会在路径前自动拼接前缀地址，使用者只需要关注模块内跳转
 * 需要在空间模块内使用
 * @example
 * const navigate = useNavigateModule();
 * //跳转到 /space/:spaceID/pe
 * navigate("pe")
 * @returns
 */
export const createUseNavigateModule =
  (useRouteInfo: UseRouteInfo): UseNavigateModule =>
  () => {
    const navigateBase = useBaseNavigate();
    const { getBaseURL } = useRouteInfo();

    const navigate = useCallback(
      (
        to: To | number,
        options?: NavigateOptions & { params?: RouteInfoURLParams },
      ) => {
        if (typeof to === 'number') {
          navigateBase(to);
        } else if (typeof to === 'string') {
          const url = getPath(to, getBaseURL(options?.params));
          navigateBase(url, options);
        } else {
          navigateBase(
            {
              ...to,
              pathname: getPath(to.pathname || '', getBaseURL(options?.params)),
            },
            options,
          );
        }
      },
      [getBaseURL],
    );

    return navigate;
  };
