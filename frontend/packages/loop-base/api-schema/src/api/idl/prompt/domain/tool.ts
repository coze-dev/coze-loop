// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const PublicDraftVersion = "$PublicDraft";
export interface Tool {
  id?: string,
  workspace_id?: string,
  tool_basic?: ToolBasic,
  tool_commit?: ToolCommit,
  ext_infos?: {
    [key: string | number]: string
  },
}
export interface ToolBasic {
  name?: string,
  description?: string,
  latest_committed_version?: string,
  created_by?: string,
  updated_by?: string,
  created_at?: string,
  updated_at?: string,
}
export interface ToolCommit {
  detail?: ToolDetail,
  commit_info?: CommitInfo,
}
export interface CommitInfo {
  version?: string,
  base_version?: string,
  description?: string,
  committed_by?: string,
  committed_at?: string,
}
export interface ToolDetail {
  content?: string,
  ext_infos?: {
    [key: string | number]: string
  },
}