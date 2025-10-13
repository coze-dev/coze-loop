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

import { useParams } from 'react-router-dom';
import { useCallback, useMemo } from 'react';

import {
  type RouteInfo,
  type UseRouteInfo,
  type RouteInfoURLParams,
} from '@cozeloop/route-base';

const PREFIX = '/console';

const getBaseURLBase = (params: RouteInfoURLParams) => {
  let baseURL = PREFIX;

  if (params.enterpriseID) {
    baseURL += `/enterprise/${params.enterpriseID}`;
  }
  if (params.organizeID) {
    baseURL += `/organize/${params.organizeID}`;
  }
  if (params.spaceID) {
    baseURL += `/space/${params.spaceID}`;
  }

  return baseURL;
};

export const useRouteInfo: UseRouteInfo = () => {
  const { enterpriseID, organizeID, spaceID } = useParams<{
    enterpriseID: string;
    spaceID: string;
    organizeID: string;
  }>();

  const { pathname } = window.location ?? {};

  const routeInfo = useMemo(() => {
    const baseURL = getBaseURLBase({
      enterpriseID,
      organizeID,
      spaceID,
    });

    const subPath = pathname.replace(baseURL, '');

    const [, app, subModule, detail] = subPath.split('/');

    return {
      baseURL,
      app,
      subModule,
      detail,
    };
  }, [pathname, enterpriseID, organizeID, spaceID]);

  const getBaseURL: RouteInfo['getBaseURL'] = useCallback(
    params =>
      getBaseURLBase({
        enterpriseID,
        organizeID,
        spaceID,
        ...params,
      }),
    [enterpriseID, organizeID, spaceID],
  );

  return {
    enterpriseID,
    organizeID,
    spaceID,
    getBaseURL,
    ...routeInfo,
  };
};
