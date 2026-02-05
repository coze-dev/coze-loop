// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as prompt from './domain/prompt';
export { prompt };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export const DebugStreaming = /*#__PURE__*/createAPI<DebugStreamingRequest, DebugStreamingResponse, {
  prompt_id: string | number;
}>({
  "url": "/api/prompt/v1/prompts/:prompt_id/debug_streaming",
  "method": "POST",
  "name": "DebugStreaming",
  "reqType": "DebugStreamingRequest",
  "reqMapping": {
    "body": ["prompt", "messages", "variable_vals", "mock_tools", "single_step_debug", "debug_trace_key"]
  },
  "resType": "DebugStreamingResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.debug",
  "service": "promptDebug"
});
export const SaveDebugContext = /*#__PURE__*/createAPI<SaveDebugContextRequest, SaveDebugContextResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/debug_context/save",
  "method": "POST",
  "name": "SaveDebugContext",
  "reqType": "SaveDebugContextRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["workspace_id", "debug_context"]
  },
  "resType": "SaveDebugContextResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.debug",
  "service": "promptDebug"
});
export const GetDebugContext = /*#__PURE__*/createAPI<GetDebugContextRequest, GetDebugContextResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/debug_context/get",
  "method": "GET",
  "name": "GetDebugContext",
  "reqType": "GetDebugContextRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "query": ["workspace_id"]
  },
  "resType": "GetDebugContextResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.debug",
  "service": "promptDebug"
});
export const ListDebugHistory = /*#__PURE__*/createAPI<ListDebugHistoryRequest, ListDebugHistoryResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/debug_history/list",
  "method": "GET",
  "name": "ListDebugHistory",
  "reqType": "ListDebugHistoryRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "query": ["workspace_id", "days_limit", "page_size", "page_token"]
  },
  "resType": "ListDebugHistoryResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.debug",
  "service": "promptDebug"
});
export interface DebugStreamingRequest {
  prompt?: prompt.Prompt,
  messages?: prompt.Message[],
  variable_vals?: prompt.VariableVal[],
  mock_tools?: prompt.MockTool[],
  single_step_debug?: boolean,
  debug_trace_key?: string,
}
export interface DebugStreamingResponse {
  delta?: prompt.Message,
  finish_reason?: string,
  usage?: prompt.TokenUsage,
  debug_id?: string,
  debug_trace_key?: string,
}
export interface SaveDebugContextRequest {
  prompt_id?: string,
  workspace_id?: string,
  debug_context?: prompt.DebugContext,
}
export interface SaveDebugContextResponse {}
export interface GetDebugContextRequest {
  prompt_id?: string,
  workspace_id?: string,
}
export interface GetDebugContextResponse {
  debug_context?: prompt.DebugContext
}
export interface ListDebugHistoryRequest {
  prompt_id?: string,
  workspace_id?: string,
  days_limit?: number,
  page_size?: number,
  page_token?: string,
}
export interface ListDebugHistoryResponse {
  debug_history?: prompt.DebugLog[],
  has_more?: boolean,
  next_page_token?: string,
}