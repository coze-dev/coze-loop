// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as common from './domain/common';
export { common };
import * as manage from './domain/manage';
export { manage };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface ListModelsRequest {
  workspace_id?: string,
  scenario?: common.Scenario,
  page_size?: number,
  page_token?: string,
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
    "body": ["workspace_id", "scenario", "page_size", "page_token"]
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
    "body": ["workspace_id"],
    "path": ["model_id"]
  },
  "resType": "GetModelResponse",
  "schemaRoot": "api://schemas/llm_coze.loop.llm.manage",
  "service": "llmManage"
});