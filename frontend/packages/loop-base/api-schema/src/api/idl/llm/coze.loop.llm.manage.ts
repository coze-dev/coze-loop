// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as common from './domain/common';
export { common };
import * as manage from './domain/manage';
export { manage };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface Filter {
  name_like?: string,
  families?: manage.Family[],
  statuses?: manage.ModelStatus[],
  abilities?: manage.AbilityEnum[],
}
export interface ListModelsRequest {
  workspace_id?: string,
  scenario?: common.Scenario,
  filter?: Filter,
  /** 是否为预置模型 */
  preset_model?: boolean,
  cookie?: string,
  page_size?: number,
  page_token?: string,
  page?: number,
}
export interface ListModelsResponse {
  models?: manage.Model[],
  has_more?: boolean,
  next_page_token?: string,
  total?: number,
}
export interface GetModelRequest {
  workspace_id?: string,
  model_id?: string,
  identification?: string,
  protocol?: manage.Protocol,
  /** 是否为预置模型 */
  preset_model?: boolean,
}
export interface GetModelResponse {
  model?: manage.Model
}
export const ListModels = /*#__PURE__*/createAPI<ListModelsRequest, ListModelsResponse>({
  "url": "/api/llm/v1/models/list",
  "method": "POST",
  "name": "ListModels",
  "reqType": "ListModelsRequest",
  "reqMapping": {
    "body": ["workspace_id", "scenario", "filter", "preset_model", "page_size", "page_token", "page"],
    "header": ["cookie"]
  },
  "resType": "ListModelsResponse",
  "schemaRoot": "api://schemas/llm_coze.loop.llm.manage",
  "service": "llmManage"
});
export const GetModel = /*#__PURE__*/createAPI<GetModelRequest, GetModelResponse>({
  "url": "/api/llm/v1/models/:model_id",
  "method": "POST",
  "name": "GetModel",
  "reqType": "GetModelRequest",
  "reqMapping": {
    "body": ["workspace_id", "identification", "protocol", "preset_model"],
    "path": ["model_id"]
  },
  "resType": "GetModelResponse",
  "schemaRoot": "api://schemas/llm_coze.loop.llm.manage",
  "service": "llmManage"
});