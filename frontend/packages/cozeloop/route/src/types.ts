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

import { type NavigateOptions, type To } from 'react-router-dom';

export interface RouteInfoURLParams {
  spaceID?: string;
  enterpriseID?: string;
  organizeID?: string;
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
   * 空间 id
   */
  spaceID?: string;
  /**
   * 企业 id
   */
  enterpriseID?: string;
  /**
   * 组织 id
   */
  organizeID?: string;
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
