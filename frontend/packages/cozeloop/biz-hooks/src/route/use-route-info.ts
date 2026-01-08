// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

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
