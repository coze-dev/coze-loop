// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as dataset from './domain/dataset';
export { dataset };
import * as tag from './domain/tag';
export { tag };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateTagRequest {
  workspace_id: string,
  tag_key_name: string,
  description?: string,
  tag_content_spec?: tag.TagContentSpec,
  tag_values?: tag.TagValue[],
  tag_domain_types?: tag.TagDomainType[],
  tag_content_type?: tag.TagContentType,
  version?: string,
  tag_type?: tag.TagType,
}
export interface CreateTagResponse {
  tag_key_id?: string
}
export interface UpdateTagRequest {
  workspace_id: string,
  tag_key_id: string,
  tag_key_name: string,
  description?: string,
  tag_content_spec?: tag.TagContentSpec,
  tag_values?: tag.TagValue[],
  tag_domain_types?: tag.TagDomainType[],
  tag_content_type?: tag.TagContentType,
  version?: string,
}
export interface UpdateTagResponse {}
export interface BatchUpdateTagStatusRequest {
  workspace_id: string,
  tag_key_ids: string[],
  to_status: tag.TagStatus,
}
export interface BatchUpdateTagStatusResponse {
  err_info?: {
    [key: string | number]: string
  }
}
export interface SearchTagsRequest {
  workspace_id: string,
  tag_key_name_like?: string,
  created_bys?: string[],
  domain_types?: tag.TagDomainType[],
  content_types?: tag.TagContentType[],
  tag_key_name?: string,
  /** pagination */
  page_number?: number,
  page_size?: number,
  page_token?: string,
  order_by?: dataset.OrderBy,
}
export interface SearchTagsResponse {
  tagInfos?: tag.TagInfo[],
  next_page_token?: string,
  total?: string,
}
export interface GetTagDetailRequest {
  workspace_id: string,
  tag_key_id: string,
  /** pagination */
  page_number?: number,
  page_size?: number,
  page_token?: string,
  order_by?: dataset.OrderBy,
}
export interface GetTagDetailResponse {
  tags?: tag.TagInfo[],
  next_page_token?: string,
  total?: string,
}
export interface GetTagSpecRequest {
  workspace_id: string
}
export interface GetTagSpecResponse {
  /** 最大高度 */
  max_height?: number,
  /** 最大宽度(一层最多有多少个) */
  max_width?: number,
  /** 最多个数(各层加一起总数) */
  max_total?: number,
}
export interface BatchGetTagsRequest {
  workspace_id: string,
  tag_key_ids: string[],
}
export interface BatchGetTagsResponse {
  tag_info_list?: tag.TagInfo[]
}
export interface ArchiveOptionTagRequest {
  workspace_id: string,
  tag_key_id: string,
  name: string,
  description?: string,
}
export interface ArchiveOptionTagResponse {}
/**
 * Tag
 * 新增标签
*/
export const CreateTag = /*#__PURE__*/createAPI<CreateTagRequest, CreateTagResponse>({
  "url": "/api/data/v1/tags",
  "method": "POST",
  "name": "CreateTag",
  "reqType": "CreateTagRequest",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_name", "description", "tag_content_spec", "tag_values", "tag_domain_types", "tag_content_type", "version", "tag_type"]
  },
  "resType": "CreateTagResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 更新标签 */
export const UpdateTag = /*#__PURE__*/createAPI<UpdateTagRequest, UpdateTagResponse>({
  "url": "/api/data/v1/tags/:tag_key_id",
  "method": "PATCH",
  "name": "UpdateTag",
  "reqType": "UpdateTagRequest",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_name", "description", "tag_content_spec", "tag_values", "tag_domain_types", "tag_content_type", "version"],
    "path": ["tag_key_id"]
  },
  "resType": "UpdateTagResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 批量更新标签状态 */
export const BatchUpdateTagStatus = /*#__PURE__*/createAPI<BatchUpdateTagStatusRequest, BatchUpdateTagStatusResponse>({
  "url": "/api/data/v1/tags/batch_update_status",
  "method": "POST",
  "name": "BatchUpdateTagStatus",
  "reqType": "BatchUpdateTagStatusRequest",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_ids", "to_status"]
  },
  "resType": "BatchUpdateTagStatusResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 搜索标签 */
export const SearchTags = /*#__PURE__*/createAPI<SearchTagsRequest, SearchTagsResponse>({
  "url": "/api/data/v1/tags/search",
  "method": "POST",
  "name": "SearchTags",
  "reqType": "SearchTagsRequest",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_name_like", "created_bys", "domain_types", "content_types", "tag_key_name", "page_number", "page_size", "page_token", "order_by"]
  },
  "resType": "SearchTagsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 标签详情 */
export const GetTagDetail = /*#__PURE__*/createAPI<GetTagDetailRequest, GetTagDetailResponse>({
  "url": "/api/data/v1/tags/:tag_key_id/detail",
  "method": "POST",
  "name": "GetTagDetail",
  "reqType": "GetTagDetailRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "page_token", "order_by"],
    "path": ["tag_key_id"]
  },
  "resType": "GetTagDetailResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 获取标签限制 */
export const GetTagSpec = /*#__PURE__*/createAPI<GetTagSpecRequest, GetTagSpecResponse>({
  "url": "/api/data/v1/tag_spec",
  "method": "GET",
  "name": "GetTagSpec",
  "reqType": "GetTagSpecRequest",
  "reqMapping": {
    "query": ["workspace_id"]
  },
  "resType": "GetTagSpecResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 批量获取标签 */
export const BatchGetTags = /*#__PURE__*/createAPI<BatchGetTagsRequest, BatchGetTagsResponse>({
  "url": "/api/data/v1/tags/batch_get",
  "method": "POST",
  "name": "BatchGetTags",
  "reqType": "BatchGetTagsRequest",
  "reqMapping": {
    "body": ["workspace_id", "tag_key_ids"]
  },
  "resType": "BatchGetTagsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});
/** 将单选标签归档进标签管理 */
export const ArchiveOptionTag = /*#__PURE__*/createAPI<ArchiveOptionTagRequest, ArchiveOptionTagResponse>({
  "url": "/api/data/v1/tags/:tag_key_id/archive_option_tag",
  "method": "POST",
  "name": "ArchiveOptionTag",
  "reqType": "ArchiveOptionTagRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "description"],
    "path": ["tag_key_id"]
  },
  "resType": "ArchiveOptionTagResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.tag",
  "service": "dateTag"
});