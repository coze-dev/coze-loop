// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as user from './domain/user';
export { user };
import * as tool from './domain/tool';
export { tool };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export const CreateTool = /*#__PURE__*/createAPI<CreateToolRequest, CreateToolResponse>({
  "url": "/api/prompt/v1/tools",
  "method": "POST",
  "name": "CreateTool",
  "reqType": "CreateToolRequest",
  "reqMapping": {
    "body": ["workspace_id", "tool_name", "tool_description", "draft_detail"]
  },
  "resType": "CreateToolResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const GetToolDetail = /*#__PURE__*/createAPI<GetToolDetailRequest, GetToolDetailResponse>({
  "url": "/api/prompt/v1/tools/:tool_id",
  "method": "GET",
  "name": "GetToolDetail",
  "reqType": "GetToolDetailRequest",
  "reqMapping": {
    "path": ["tool_id"],
    "query": ["workspace_id", "with_commit", "commit_version", "with_draft"]
  },
  "resType": "GetToolDetailResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const ListTool = /*#__PURE__*/createAPI<ListToolRequest, ListToolResponse>({
  "url": "/api/prompt/v1/tools/list",
  "method": "POST",
  "name": "ListTool",
  "reqType": "ListToolRequest",
  "reqMapping": {
    "body": ["workspace_id", "key_word", "created_bys", "committed_only", "page_num", "page_size", "order_by", "asc"]
  },
  "resType": "ListToolResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const SaveToolDetail = /*#__PURE__*/createAPI<SaveToolDetailRequest, SaveToolDetailResponse>({
  "url": "/api/prompt/v1/tools/:tool_id/drafts/save",
  "method": "POST",
  "name": "SaveToolDetail",
  "reqType": "SaveToolDetailRequest",
  "reqMapping": {
    "path": ["tool_id"],
    "query": ["workspace_id"],
    "body": ["tool_detail", "base_version"]
  },
  "resType": "SaveToolDetailResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const CommitToolDraft = /*#__PURE__*/createAPI<CommitToolDraftRequest, CommitToolDraftResponse>({
  "url": "/api/prompt/v1/tools/:tool_id/drafts/commit",
  "method": "POST",
  "name": "CommitToolDraft",
  "reqType": "CommitToolDraftRequest",
  "reqMapping": {
    "path": ["tool_id"],
    "query": ["workspace_id"],
    "body": ["commit_version", "commit_description", "base_version"]
  },
  "resType": "CommitToolDraftResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const ListToolCommit = /*#__PURE__*/createAPI<ListToolCommitRequest, ListToolCommitResponse>({
  "url": "/api/prompt/v1/tools/:tool_id/commits/list",
  "method": "POST",
  "name": "ListToolCommit",
  "reqType": "ListToolCommitRequest",
  "reqMapping": {
    "path": ["tool_id"],
    "query": ["workspace_id", "with_commit_detail"],
    "body": ["page_size", "page_token", "asc"]
  },
  "resType": "ListToolCommitResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export const BatchGetTools = /*#__PURE__*/createAPI<BatchGetToolsRequest, BatchGetToolsResponse>({
  "url": "/api/prompt/v1/tools/mget",
  "method": "POST",
  "name": "BatchGetTools",
  "reqType": "BatchGetToolsRequest",
  "reqMapping": {
    "body": ["workspace_id", "queries"]
  },
  "resType": "BatchGetToolsResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.tool_manage",
  "service": "toolManage"
});
export interface CreateToolRequest {
  workspace_id?: string,
  tool_name?: string,
  tool_description?: string,
  draft_detail?: tool.ToolDetail,
}
export interface CreateToolResponse {
  tool_id?: string
}
export interface GetToolDetailRequest {
  tool_id?: string,
  workspace_id?: string,
  with_commit?: boolean,
  commit_version?: string,
  with_draft?: boolean,
}
export interface GetToolDetailResponse {
  tool?: tool.Tool
}
export interface ListToolRequest {
  workspace_id?: string,
  key_word?: string,
  created_bys?: string[],
  committed_only?: boolean,
  page_num?: number,
  page_size?: number,
  order_by?: ListToolOrderBy,
  asc?: boolean,
}
export interface ListToolResponse {
  tools?: tool.Tool[],
  users?: user.UserInfoDetail[],
  total?: number,
}
export enum ListToolOrderBy {
  CommittedAt = "committed_at",
  CreatedAt = "created_at",
}
export interface SaveToolDetailRequest {
  tool_id?: string,
  workspace_id?: string,
  tool_detail?: tool.ToolDetail,
  base_version?: string,
}
export interface SaveToolDetailResponse {}
export interface CommitToolDraftRequest {
  tool_id?: string,
  workspace_id?: string,
  commit_version?: string,
  commit_description?: string,
  base_version?: string,
}
export interface CommitToolDraftResponse {}
export interface ListToolCommitRequest {
  tool_id?: string,
  workspace_id?: string,
  with_commit_detail?: boolean,
  page_size?: number,
  page_token?: string,
  asc?: boolean,
}
export interface ListToolCommitResponse {
  tool_commit_infos?: tool.CommitInfo[],
  tool_commit_detail_mapping?: {
    [key: string | number]: tool.ToolDetail
  },
  users?: user.UserInfoDetail[],
  has_more?: boolean,
  next_page_token?: string,
}
export interface ToolQuery {
  tool_id?: string,
  version?: string,
}
export interface ToolResult {
  query?: ToolQuery,
  tool?: tool.Tool,
}
export interface BatchGetToolsRequest {
  workspace_id?: string,
  queries?: ToolQuery[],
}
export interface BatchGetToolsResponse {
  items?: ToolResult[]
}