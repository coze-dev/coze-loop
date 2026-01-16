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
  if (params.organizationID) {
    baseURL += `/organization/${params.organizationID}`;
  }
  if (params.spaceID) {
    baseURL += `/space/${params.spaceID}`;
  }

  return baseURL;
};

export const useRouteInfo: UseRouteInfo = () => {
  const { enterpriseID, organizationID, spaceID } = useParams<{
    enterpriseID: string;
    spaceID: string;
    organizationID: string;
  }>();

  const { pathname } = window.location ?? {};

  const routeInfo = useMemo(() => {
    const baseURL = getBaseURLBase({
      enterpriseID,
      organizationID,
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
  }, [pathname, enterpriseID, organizationID, spaceID]);

  const getBaseURL: RouteInfo['getBaseURL'] = useCallback(
    params =>
      getBaseURLBase({
        enterpriseID,
        organizationID,
        spaceID,
        ...params,
      }),
    [enterpriseID, organizationID, spaceID],
  );

  return {
    enterpriseID,
    organizationID,
    spaceID,
    getBaseURL,
    ...routeInfo,
  };
};
