// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type NavigateOptions, type To } from 'react-router-dom';

export interface RouteInfoURLParams {
  spaceID?: string;
  enterpriseID?: string;
  organizationID?: string;
}

/**
 * 根据url能够解析出的信息
 */
export interface RouteInfo {
  /**
   * 获取基础路径，根据传入的参数拼接，如果不传入，默认使用当前空间
   * @param params
   * @returns
   */
  getBaseURL: (params?: RouteInfoURLParams) => string;
  /**
   * 业务模块
   */
  app: string;
  /**
   * 业务子模块
   */
  subModule: string;
  /**
   * 业务详情，通常为详情页
   */
  detail: string;
  /**
   * URL 中获取的空间 id
   */
  spaceID?: string;
  /**
   * URL 中获取的企业 id
   */
  enterpriseID?: string;
  /**
   * URL 中获取的组织 id
   */
  organizationID?: string;
}

/**
 * 基于路由获取信息
 */
export type UseRouteInfo = () => RouteInfo;

/**
 * 通用路由跳转，屏蔽业务差异
 */
export type UseNavigateModule = () => (
  to: To | number,
  options?: NavigateOptions & { params?: RouteInfoURLParams },
) => void;

/**
 * 统一的打开链接逻辑
 */
export type UseOpenWindow = () => {
  openBlank: (url: string, params?: RouteInfoURLParams) => void;
  openSelf: (url: string, params?: RouteInfoURLParams) => void;
  getURL: (path: string, params?: RouteInfoURLParams) => string;
};
