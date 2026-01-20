// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as user from './domain/user';
export { user };
import * as prompt from './domain/prompt';
export { prompt };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
/**
 * --------------- Prompt管理 --------------- //
 * 增
*/
export const CreatePrompt = /*#__PURE__*/createAPI<CreatePromptRequest, CreatePromptResponse>({
  "url": "/api/prompt/v1/prompts",
  "method": "POST",
  "name": "CreatePrompt",
  "reqType": "CreatePromptRequest",
  "reqMapping": {
    "body": ["workspace_id", "prompt_name", "prompt_key", "prompt_description", "prompt_type", "draft_detail"]
  },
  "resType": "CreatePromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const ClonePrompt = /*#__PURE__*/createAPI<ClonePromptRequest, ClonePromptResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/clone",
  "method": "POST",
  "name": "ClonePrompt",
  "reqType": "ClonePromptRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["commit_version", "cloned_prompt_name", "cloned_prompt_key", "cloned_prompt_description"]
  },
  "resType": "ClonePromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** 删 */
export const DeletePrompt = /*#__PURE__*/createAPI<DeletePromptRequest, DeletePromptResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id",
  "method": "DELETE",
  "name": "DeletePrompt",
  "reqType": "DeletePromptRequest",
  "reqMapping": {
    "path": ["prompt_id"]
  },
  "resType": "DeletePromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** 查 */
export const GetPrompt = /*#__PURE__*/createAPI<GetPromptRequest, GetPromptResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id",
  "method": "GET",
  "name": "GetPrompt",
  "reqType": "GetPromptRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "query": ["workspace_id", "with_commit", "commit_version", "with_draft", "with_default_config", "expand_snippet"]
  },
  "resType": "GetPromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const ListPrompt = /*#__PURE__*/createAPI<ListPromptRequest, ListPromptResponse>({
  "url": "/api/prompt/v1/prompts/list",
  "method": "POST",
  "name": "ListPrompt",
  "reqType": "ListPromptRequest",
  "reqMapping": {
    "body": ["workspace_id", "key_word", "created_bys", "committed_only", "filter_prompt_types", "page_num", "page_size", "order_by", "asc"]
  },
  "resType": "ListPromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** 查询片段的引用记录 */
export const ListParentPrompt = /*#__PURE__*/createAPI<ListParentPromptRequest, ListParentPromptResponse>({
  "url": "/api/prompt/v1/prompts/list_parent",
  "method": "POST",
  "name": "ListParentPrompt",
  "reqType": "ListParentPromptRequest",
  "reqMapping": {
    "body": ["workspace_id", "prompt_id", "commit_versions"]
  },
  "resType": "ListParentPromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** 改 */
export const UpdatePrompt = /*#__PURE__*/createAPI<UpdatePromptRequest, UpdatePromptResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id",
  "method": "PUT",
  "name": "UpdatePrompt",
  "reqType": "UpdatePromptRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["prompt_name", "prompt_description"]
  },
  "resType": "UpdatePromptResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const SaveDraft = /*#__PURE__*/createAPI<SaveDraftRequest, SaveDraftResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/drafts/save",
  "method": "POST",
  "name": "SaveDraft",
  "reqType": "SaveDraftRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["prompt_draft"]
  },
  "resType": "SaveDraftResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/**
 * --------------- Label管理 --------------- //
 * Label管理
*/
export const CreateLabel = /*#__PURE__*/createAPI<CreateLabelRequest, CreateLabelResponse>({
  "url": "/api/prompt/v1/labels",
  "method": "POST",
  "name": "CreateLabel",
  "reqType": "CreateLabelRequest",
  "reqMapping": {
    "body": ["workspace_id", "label"]
  },
  "resType": "CreateLabelResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const ListLabel = /*#__PURE__*/createAPI<ListLabelRequest, ListLabelResponse>({
  "url": "/api/prompt/v1/labels/list",
  "method": "POST",
  "name": "ListLabel",
  "reqType": "ListLabelRequest",
  "reqMapping": {
    "body": ["workspace_id", "label_key_like", "with_prompt_version_mapping", "prompt_id", "page_size", "page_token"]
  },
  "resType": "ListLabelResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const BatchGetLabel = /*#__PURE__*/createAPI<BatchGetLabelRequest, BatchGetLabelResponse>({
  "url": "/api/prompt/v1/labels/batch_get",
  "method": "POST",
  "name": "BatchGetLabel",
  "reqType": "BatchGetLabelRequest",
  "reqMapping": {
    "body": ["workspace_id", "label_keys"]
  },
  "resType": "BatchGetLabelResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** --------------- Prompt版本管理 --------------- // */
export const ListCommit = /*#__PURE__*/createAPI<ListCommitRequest, ListCommitResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/commits/list",
  "method": "POST",
  "name": "ListCommit",
  "reqType": "ListCommitRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "query": ["with_commit_detail"],
    "body": ["page_size", "page_token", "asc"]
  },
  "resType": "ListCommitResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const CommitDraft = /*#__PURE__*/createAPI<CommitDraftRequest, CommitDraftResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/drafts/commit",
  "method": "POST",
  "name": "CommitDraft",
  "reqType": "CommitDraftRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["commit_version", "commit_description", "label_keys"]
  },
  "resType": "CommitDraftResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const RevertDraftFromCommit = /*#__PURE__*/createAPI<RevertDraftFromCommitRequest, RevertDraftFromCommitResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/drafts/revert_from_commit",
  "method": "POST",
  "name": "RevertDraftFromCommit",
  "reqType": "RevertDraftFromCommitRequest",
  "reqMapping": {
    "path": ["prompt_id"],
    "body": ["commit_version_reverting_from"]
  },
  "resType": "RevertDraftFromCommitResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
export const UpdateCommitLabels = /*#__PURE__*/createAPI<UpdateCommitLabelsRequest, UpdateCommitLabelsResponse>({
  "url": "/api/prompt/v1/prompts/:prompt_id/commits/:commit_version/labels_update",
  "method": "POST",
  "name": "UpdateCommitLabels",
  "reqType": "UpdateCommitLabelsRequest",
  "reqMapping": {
    "body": ["workspace_id", "label_keys"],
    "path": ["prompt_id", "commit_version"]
  },
  "resType": "UpdateCommitLabelsResponse",
  "schemaRoot": "api://schemas/prompt_coze.loop.prompt.manage",
  "service": "promptManage"
});
/** --------------- Prompt管理 --------------- // */
export interface CreatePromptRequest {
  workspace_id?: string,
  prompt_name?: string,
  prompt_key?: string,
  prompt_description?: string,
  prompt_type?: prompt.PromptType,
  draft_detail?: prompt.PromptDetail,
}
export interface CreatePromptResponse {
  prompt_id?: string
}
export interface ClonePromptRequest {
  prompt_id?: string,
  commit_version?: string,
  cloned_prompt_name?: string,
  cloned_prompt_key?: string,
  cloned_prompt_description?: string,
}
export interface ClonePromptResponse {
  cloned_prompt_id?: string
}
export interface DeletePromptRequest {
  prompt_id?: string
}
export interface DeletePromptResponse {}
export interface GetPromptRequest {
  prompt_id?: string,
  workspace_id?: string,
  with_commit?: boolean,
  commit_version?: string,
  with_draft?: boolean,
  with_default_config?: boolean,
  /** 是否展开子片段，true:展开 */
  expand_snippet?: boolean,
}
export interface GetPromptResponse {
  prompt?: prompt.Prompt,
  default_config?: prompt.PromptDetail,
  /** [片段]被引用的总数 */
  total_parent_references?: number,
}
export interface PromptQuery {
  prompt_id?: string,
  with_commit?: boolean,
  commit_version?: string,
}
export interface BatchGetPromptRequest {
  queries?: PromptQuery[]
}
export interface BatchGetPromptResponse {
  results?: PromptResult[]
}
export interface PromptResult {
  query?: PromptQuery,
  prompt?: prompt.Prompt,
}
export interface ListPromptRequest {
  workspace_id?: string,
  key_word?: string,
  created_bys?: string[],
  committed_only?: boolean,
  /** 向前兼容，如果不传，默认查询normal类型的Prompt */
  filter_prompt_types?: prompt.PromptType[],
  page_num?: number,
  page_size?: number,
  order_by?: ListPromptOrderBy,
  asc?: boolean,
}
export interface ListPromptResponse {
  prompts?: prompt.Prompt[],
  users?: user.UserInfoDetail[],
  total?: number,
}
export enum ListPromptOrderBy {
  CommitedAt = "committed_at",
  CreatedAt = "created_at",
}
export interface UpdatePromptRequest {
  prompt_id?: string,
  prompt_name?: string,
  prompt_description?: string,
}
export interface UpdatePromptResponse {}
export interface SaveDraftRequest {
  prompt_id?: string,
  prompt_draft?: prompt.PromptDraft,
}
export interface SaveDraftResponse {
  draft_info?: prompt.DraftInfo
}
export interface CommitDraftRequest {
  prompt_id?: string,
  commit_version?: string,
  commit_description?: string,
  label_keys?: string[],
}
export interface CommitDraftResponse {}
/** 搜索Prompt提交版本 */
export interface ListCommitRequest {
  prompt_id?: string,
  /** 是否查询详情 */
  with_commit_detail?: boolean,
  page_size?: number,
  page_token?: string,
  asc?: boolean,
}
export interface ListCommitResponse {
  prompt_commit_infos?: prompt.CommitInfo[],
  commit_version_label_mapping?: {
    [key: string | number]: prompt.Label[]
  },
  /** key: version, value:被引用数 */
  parent_references_mapping?: {
    [key: string | number]: number
  },
  /** key:version, value:PromptDetail */
  prompt_commit_detail_mapping?: {
    [key: string | number]: prompt.PromptDetail
  },
  users?: user.UserInfoDetail[],
  has_more?: boolean,
  next_page_token?: string,
}
export interface RevertDraftFromCommitRequest {
  prompt_id?: string,
  commit_version_reverting_from?: string,
}
export interface RevertDraftFromCommitResponse {}
/** --------------- Label管理相关结构体 --------------- // */
export interface CreateLabelRequest {
  workspace_id?: string,
  label?: prompt.Label,
}
export interface CreateLabelResponse {}
export interface ListLabelRequest {
  workspace_id?: string,
  /** 模糊匹配label key */
  label_key_like?: string,
  with_prompt_version_mapping?: boolean,
  prompt_id?: string,
  page_size?: number,
  page_token?: string,
}
export interface ListLabelResponse {
  labels?: prompt.Label[],
  prompt_version_mapping?: {
    [key: string | number]: string
  },
  has_more?: boolean,
  next_page_token?: string,
}
export interface BatchGetLabelRequest {
  workspace_id?: string,
  label_keys?: string[],
}
export interface BatchGetLabelResponse {
  labels?: prompt.Label[]
}
export interface UpdateCommitLabelsRequest {
  workspace_id?: string,
  prompt_id?: string,
  commit_version?: string,
  label_keys?: string[],
}
export interface UpdateCommitLabelsResponse {}
export interface ListParentPromptRequest {
  workspace_id?: string,
  prompt_id?: string,
  /** 片段版本，不传则表示查询所有版本的引用记录 */
  commit_versions?: string[],
}
export interface ListParentPromptResponse {
  /** 不同片段版本被引用的父prompt记录 */
  parent_prompts?: {
    [key: string | number]: prompt.PromptCommitVersions[]
  }
}