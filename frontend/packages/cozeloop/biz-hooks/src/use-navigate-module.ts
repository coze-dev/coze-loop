// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  useNavigate,
  type NavigateOptions,
  type To,
  type NavigateFunction,
  useParams,
} from 'react-router-dom';
import { useCallback } from 'react';

import { useSpace } from './use-space';

/**
 * 基于模块的 navigate，会在路径前自动拼接 /console/space/:spaceID/ 或者 /console/enterprise/:enterpriseID/space/:spaceID/
 * 需要在空间模块内使用
 * @example
 * const navigate = useNavigateModule();
 * //跳转到 /console/space/:spaceID/pe
 * navigate("pe")
 * @returns
 */
export function useNavigateModule(): NavigateFunction {
  const navigate = useNavigate();
  const { baseURL } = useBaseURL();
  return useCallback(
    (to: To | number, options?: NavigateOptions) => {
      if (typeof to === 'number') {
        navigate(to);
      } else {
        if (typeof to === 'string') {
          navigate(`${baseURL}/${to}`, options);
        } else {
          navigate(
            {
              ...to,
              pathname: `${baseURL}/${to.pathname}`,
            },
            options,
          );
        }
      }
    },
    [baseURL],
  );
}

interface BaseURLProps {
  /** 默认的企业 ID */
  enterpriseID?: string;
  /** 默认的空间 ID */
  spaceID?: string;
}

/**
 * 基于模块的 navigate，会在路径前自动拼接 /console/space/:spaceID 或者 /console/enterprise/:enterpriseID/space/:spaceID
 * 需要在空间模块内使用
 */
export function useBaseURL() {
  const { enterpriseID: enterpriseIDFromParams, spaceID: spaceIDFromParams } =
    useParams<{
      enterpriseID: string;
      spaceID: string;
    }>();

  const { space } = useSpace();

  const getBaseURL = useCallback(
    (params: BaseURLProps = {}) => {
      const enterpriseID = params.enterpriseID ?? enterpriseIDFromParams;
      const spaceID = params.spaceID ?? spaceIDFromParams ?? space?.id;

      return `/console${enterpriseID ? `/enterprise/${enterpriseID}` : ''}/space/${spaceID}`;
    },
    [enterpriseIDFromParams, spaceIDFromParams, space?.id],
  );

  const getBasePrefix = useCallback(
    (params: BaseURLProps = {}) => {
      const enterpriseID = params.enterpriseID ?? enterpriseIDFromParams;
      return `/console${enterpriseID ? `/enterprise/${enterpriseID}` : ''}`;
    },
    [enterpriseIDFromParams],
  );
  return {
    getBaseURL,
    baseURL: getBaseURL(),
    getBasePrefix,
  };
}

/**
 * 获取coze 相关地址信息
 * @returns
 */
export function useCozeLocation() {
  const cozeOrigin = window.location.origin.replace('loop.', '');

  return {
    origin: cozeOrigin,
  };
}
