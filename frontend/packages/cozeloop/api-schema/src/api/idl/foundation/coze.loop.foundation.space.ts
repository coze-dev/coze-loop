// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as space from './domain/space';
export { space };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
/** 查询空间信息 */
export interface GetSpaceRequest {
  space_id: number
}
export interface GetSpaceResponse {
  space?: space.Space
}
/** 空间列表: 用户有权限的空间列表 */
export interface ListUserSpaceRequest {
  user_id?: string,
  /** 分页数量 */
  page_size?: number,
  /** 当前请求页码，当有page_token字段时，会忽略该字段，默认按照page_token分页查询数据 */
  page_number?: number,
}
export interface ListUserSpaceResponse {
  /** 空间列表 */
  spaces?: space.Space[],
  /** 空间总数 */
  total?: number,
}
/** 查询空间信息 */
export const GetSpace = /*#__PURE__*/createAPI<GetSpaceRequest, GetSpaceResponse>({
  "url": "/api/foundation/v1/spaces/:space_id",
  "method": "GET",
  "name": "GetSpace",
  "reqType": "GetSpaceRequest",
  "reqMapping": {
    "path": ["space_id"]
  },
  "resType": "GetSpaceResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.space",
  "service": "foundationSpace"
});
/** 空间列表 */
export const ListUserSpaces = /*#__PURE__*/createAPI<ListUserSpaceRequest, ListUserSpaceResponse>({
  "url": "/api/foundation/v1/spaces/list",
  "method": "POST",
  "name": "ListUserSpaces",
  "reqType": "ListUserSpaceRequest",
  "reqMapping": {
    "body": ["user_id", "page_size", "page_number"]
  },
  "resType": "ListUserSpaceResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.space",
  "service": "foundationSpace"
});