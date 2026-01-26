// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as authn from './domain/authn';
export { authn };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreatePersonalAccessTokenRequest {
  /** PAT名称 */
  name: string,
  /** PAT自定义过期时间unix，秒 */
  expire_at?: number,
  /** PAT用户枚举过期时间 1、30、60、90、180、365、permanent */
  duration_day?: DurationDay,
}
export enum DurationDay {
  Day1 = "1",
  Day30 = "30",
  Day60 = "60",
  Day90 = "90",
  Day180 = "180",
  Day365 = "365",
  Permanent = "permanent",
}
export interface CreatePersonalAccessTokenResponse {
  personal_access_token?: authn.PersonalAccessToken,
  /** PAT token 明文 */
  token?: string,
}
export interface DeletePersonalAccessTokenRequest {
  /** PAT id */
  id: string
}
export interface DeletePersonalAccessTokenResponse {}
export interface GetPersonalAccessTokenRequest {
  /** PAT Id */
  id: string
}
export interface GetPersonalAccessTokenResponse {
  personal_access_token?: authn.PersonalAccessToken
}
export interface ListPersonalAccessTokenRequest {
  /** per page size */
  page_size?: number,
  /** page number */
  page_number?: number,
}
export interface ListPersonalAccessTokenResponse {
  personal_access_tokens?: authn.PersonalAccessToken[]
}
export interface UpdatePersonalAccessTokenRequest {
  /** PAT Id */
  id: string,
  /** PAT 名称 */
  name: string,
}
export interface UpdatePersonalAccessTokenResponse {}
export interface VerifyTokenRequest {
  token: string
}
export interface VerifyTokenResponse {
  valid?: boolean,
  user_id?: string,
}
/** OpenAPI PAT管理 */
export const CreatePersonalAccessToken = /*#__PURE__*/createAPI<CreatePersonalAccessTokenRequest, CreatePersonalAccessTokenResponse>({
  "url": "/api/auth/v1/personal_access_tokens",
  "method": "POST",
  "name": "CreatePersonalAccessToken",
  "reqType": "CreatePersonalAccessTokenRequest",
  "reqMapping": {
    "body": ["name", "expire_at", "duration_day"]
  },
  "resType": "CreatePersonalAccessTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.authn",
  "service": "foundationAuthn"
});
export const DeletePersonalAccessToken = /*#__PURE__*/createAPI<DeletePersonalAccessTokenRequest, DeletePersonalAccessTokenResponse>({
  "url": "/api/auth/v1/personal_access_tokens/:id",
  "method": "DELETE",
  "name": "DeletePersonalAccessToken",
  "reqType": "DeletePersonalAccessTokenRequest",
  "reqMapping": {
    "path": ["id"]
  },
  "resType": "DeletePersonalAccessTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.authn",
  "service": "foundationAuthn"
});
export const UpdatePersonalAccessToken = /*#__PURE__*/createAPI<UpdatePersonalAccessTokenRequest, UpdatePersonalAccessTokenResponse>({
  "url": "/api/auth/v1/personal_access_tokens/:id",
  "method": "PUT",
  "name": "UpdatePersonalAccessToken",
  "reqType": "UpdatePersonalAccessTokenRequest",
  "reqMapping": {
    "path": ["id"],
    "body": ["name"]
  },
  "resType": "UpdatePersonalAccessTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.authn",
  "service": "foundationAuthn"
});
export const GetPersonalAccessToken = /*#__PURE__*/createAPI<GetPersonalAccessTokenRequest, GetPersonalAccessTokenResponse>({
  "url": "/api/auth/v1/personal_access_tokens/:id",
  "method": "GET",
  "name": "GetPersonalAccessToken",
  "reqType": "GetPersonalAccessTokenRequest",
  "reqMapping": {
    "path": ["id"]
  },
  "resType": "GetPersonalAccessTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.authn",
  "service": "foundationAuthn"
});
export const ListPersonalAccessToken = /*#__PURE__*/createAPI<ListPersonalAccessTokenRequest, ListPersonalAccessTokenResponse>({
  "url": "/api/auth/v1/personal_access_tokens/list",
  "method": "POST",
  "name": "ListPersonalAccessToken",
  "reqType": "ListPersonalAccessTokenRequest",
  "reqMapping": {
    "query": ["page_size", "page_number"]
  },
  "resType": "ListPersonalAccessTokenResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.authn",
  "service": "foundationAuthn"
});