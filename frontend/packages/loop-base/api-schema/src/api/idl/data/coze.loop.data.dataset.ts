// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as dataset_job from './domain/dataset_job';
export { dataset_job };
import * as dataset from './domain/dataset';
export { dataset };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateDatasetRequest {
  workspace_id: string,
  app_id?: string,
  name: string,
  description?: string,
  category?: dataset.DatasetCategory,
  biz_category?: string,
  fields?: dataset.FieldSchema[],
  security_level?: dataset.SecurityLevel,
  visibility?: dataset.DatasetVisibility,
  spec?: dataset.DatasetSpec,
  features?: dataset.DatasetFeatures,
}
export interface CreateDatasetResponse {
  dataset_id?: string
}
export interface UpdateDatasetRequest {
  workspace_id?: string,
  dataset_id: string,
  name?: string,
  description?: string,
}
export interface UpdateDatasetResponse {}
export interface DeleteDatasetRequest {
  workspace_id?: string,
  dataset_id: string,
}
export interface DeleteDatasetResponse {}
export interface GetDatasetRequest {
  workspace_id?: string,
  dataset_id: string,
  /** 数据集已删除时是否返回 */
  with_deleted?: boolean,
}
export interface GetDatasetResponse {
  dataset?: dataset.Dataset
}
export interface BatchGetDatasetsRequest {
  workspace_id: string,
  dataset_ids: string[],
  with_deleted?: boolean,
}
export interface BatchGetDatasetsResponse {
  datasets?: dataset.Dataset[]
}
export interface ListDatasetsRequest {
  workspace_id: string,
  dataset_ids?: string[],
  category?: dataset.DatasetCategory,
  /** 支持模糊搜索 */
  name?: string,
  created_bys?: string[],
  biz_categorys?: string[],
  /** pagination */
  page_number?: number,
  /** 分页大小(0, 200]，默认为 20 */
  page_size?: number,
  /** 与 page 同时提供时，优先使用 cursor */
  page_token?: string,
  order_bys?: dataset.OrderBy[],
}
export interface ListDatasetsResponse {
  datasets?: dataset.Dataset[],
  /** pagination */
  next_page_token?: string,
  total?: string,
}
export interface SignUploadFileTokenRequest {
  workspace_id?: string,
  /** 支持 ImageX, TOS */
  storage?: dataset.StorageProvider,
  file_name?: string,
}
export interface SignUploadFileTokenResponse {
  url?: string,
  token?: dataset.FileUploadToken,
  image_x_service_id?: string,
}
export interface ImportDatasetRequest {
  workspace_id?: string,
  dataset_id: string,
  file?: dataset_job.DatasetIOFile,
  field_mappings?: dataset_job.FieldMapping[],
  option?: dataset_job.DatasetIOJobOption,
}
export interface ImportDatasetResponse {
  job_id?: string
}
export interface GetDatasetIOJobRequest {
  workspace_id?: string,
  job_id: string,
}
export interface GetDatasetIOJobResponse {
  job?: dataset_job.DatasetIOJob
}
export interface ListDatasetIOJobsRequest {
  workspace_id?: string,
  dataset_id: string,
  types?: dataset_job.JobType[],
  statuses?: dataset_job.JobStatus[],
}
export interface ListDatasetIOJobsResponse {
  jobs?: dataset_job.DatasetIOJob[]
}
export interface ListDatasetVersionsRequest {
  workspace_id?: string,
  dataset_id: string,
  /** 根据版本号模糊匹配 */
  version_like?: string,
  /** pagination */
  page_number?: number,
  /** 分页大小(0, 200]，默认为 20 */
  page_size?: number,
  /** 与 page 同时提供时，优先使用 cursor */
  page_token?: string,
  order_bys?: dataset.OrderBy[],
}
export interface ListDatasetVersionsResponse {
  versions?: dataset.DatasetVersion[],
  /** pagination */
  next_page_token?: string,
  total?: string,
}
export interface GetDatasetVersionRequest {
  workspace_id?: string,
  version_id: string,
  /** 是否返回已删除的数据，默认不返回 */
  with_deleted?: boolean,
}
export interface GetDatasetVersionResponse {
  version?: dataset.DatasetVersion,
  dataset?: dataset.Dataset,
}
export interface VersionedDataset {
  version?: dataset.DatasetVersion,
  dataset?: dataset.Dataset,
}
export interface BatchGetDatasetVersionsRequest {
  workspace_id?: string,
  version_ids: string[],
  /** 是否返回已删除的数据，默认不返回 */
  with_deleted?: boolean,
}
export interface BatchGetDatasetVersionsResponse {
  versioned_dataset?: VersionedDataset[]
}
export interface CreateDatasetVersionRequest {
  workspace_id?: string,
  dataset_id: string,
  /** 展示的版本号，SemVer2 三段式，需要大于上一版本 */
  version: string,
  desc?: string,
}
export interface CreateDatasetVersionResponse {
  id?: string
}
export interface UpdateDatasetSchemaRequest {
  workspace_id?: string,
  dataset_id: string,
  /**
   * fieldSchema.key 为空时：插入新的一列
   * fieldSchema.key 不为空时：更新对应的列
  */
  fields?: dataset.FieldSchema[],
}
export interface UpdateDatasetSchemaResponse {}
export interface GetDatasetSchemaRequest {
  workspace_id?: string,
  dataset_id: string,
  /** 是否获取已经删除的列，默认不返回 */
  with_deleted?: boolean,
}
export interface GetDatasetSchemaResponse {
  fields?: dataset.FieldSchema[]
}
export interface ValidateDatasetItemsReq {
  workspace_id?: string,
  items?: dataset.DatasetItem[],
  /** 添加到已有数据集时提供 */
  dataset_id?: string,
  /** 新建数据集并添加数据时提供 */
  dataset_category?: dataset.DatasetCategory,
  /** 新建数据集并添加数据时，必须提供；添加到已有数据集时，如非空，则覆盖已有 schema 用于校验 */
  dataset_fields?: dataset.FieldSchema[],
  /** 添加到已有数据集时，现有数据条数，做容量校验时不做考虑，仅考虑提供 items 数量是否超限 */
  ignore_current_item_count?: boolean,
}
export interface ValidateDatasetItemsResp {
  /** 合法的 item 索引，与 ValidateCreateDatasetItemsReq.items 中的索引对应 */
  valid_item_indices?: number[],
  errors?: dataset.ItemErrorGroup[],
}
export interface BatchCreateDatasetItemsRequest {
  workspace_id?: string,
  dataset_id: string,
  items?: dataset.DatasetItem[],
  /** items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据 */
  skip_invalid_items?: boolean,
  /** 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条 */
  allow_partial_add?: boolean,
}
export interface BatchCreateDatasetItemsResponse {
  /** key: item 在 items 中的索引 */
  added_items?: {
    [key: string | number]: string
  },
  errors?: dataset.ItemErrorGroup[],
}
export interface UpdateDatasetItemRequest {
  workspace_id?: string,
  dataset_id: string,
  item_id: string,
  /** 单轮数据内容，当数据集为单轮时，写入此处的值 */
  data?: dataset.FieldData[],
  /** 多轮对话数据内容，当数据集为多轮对话时，写入此处的值 */
  repeated_data?: dataset.ItemData[],
}
export interface UpdateDatasetItemResponse {}
export interface DeleteDatasetItemRequest {
  workspace_id?: string,
  dataset_id: string,
  item_id: string,
}
export interface DeleteDatasetItemResponse {}
export interface BatchDeleteDatasetItemsRequest {
  workspace_id?: string,
  dataset_id: string,
  item_ids?: string[],
}
export interface BatchDeleteDatasetItemsResponse {}
export interface ListDatasetItemsRequest {
  workspace_id?: string,
  dataset_id: string,
  /** pagination */
  page_number?: number,
  /** 分页大小(0, 200]，默认为 20 */
  page_size?: number,
  /** 与 page 同时提供时，优先使用 cursor */
  page_token?: string,
  order_bys?: dataset.OrderBy[],
}
export interface ListDatasetItemsResponse {
  items?: dataset.DatasetItem[],
  /** pagination */
  next_page_token?: string,
  total?: string,
}
export interface ListDatasetItemsByVersionRequest {
  workspace_id?: string,
  dataset_id: string,
  version_id: string,
  /** pagination */
  page_number?: number,
  /** 分页大小(0, 200]，默认为 20 */
  page_size?: number,
  /** 与 page 同时提供时，优先使用 cursor */
  page_token?: string,
  order_bys?: dataset.OrderBy[],
}
export interface ListDatasetItemsByVersionResponse {
  items?: dataset.DatasetItem[],
  /** pagination */
  next_page_token?: string,
  total?: string,
}
export interface GetDatasetItemRequest {
  workspace_id?: string,
  dataset_id: string,
  item_id: string,
}
export interface GetDatasetItemResponse {
  item?: dataset.DatasetItem
}
export interface BatchGetDatasetItemsRequest {
  workspace_id?: string,
  dataset_id: string,
  item_ids: string[],
}
export interface BatchGetDatasetItemsResponse {
  items?: dataset.DatasetItem[]
}
export interface BatchGetDatasetItemsByVersionRequest {
  workspace_id?: string,
  dataset_id: string,
  version_id: string,
  item_ids: string[],
}
export interface BatchGetDatasetItemsByVersionResponse {
  items?: dataset.DatasetItem[]
}
export interface ClearDatasetItemRequest {
  workspace_id?: string,
  dataset_id: string,
}
export interface ClearDatasetItemResponse {}
/**
 * Dataset
 * 新增数据集
*/
export const CreateDataset = /*#__PURE__*/createAPI<CreateDatasetRequest, CreateDatasetResponse>({
  "url": "/api/data/v1/datasets",
  "method": "POST",
  "name": "CreateDataset",
  "reqType": "CreateDatasetRequest",
  "reqMapping": {
    "body": ["workspace_id", "app_id", "name", "description", "category", "biz_category", "fields", "security_level", "visibility", "spec", "features"]
  },
  "resType": "CreateDatasetResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 修改数据集 */
export const UpdateDataset = /*#__PURE__*/createAPI<UpdateDatasetRequest, UpdateDatasetResponse>({
  "url": "/api/data/v1/datasets/:dataset_id",
  "method": "PATCH",
  "name": "UpdateDataset",
  "reqType": "UpdateDatasetRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "description"],
    "path": ["dataset_id"]
  },
  "resType": "UpdateDatasetResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 删除数据集 */
export const DeleteDataset = /*#__PURE__*/createAPI<DeleteDatasetRequest, DeleteDatasetResponse>({
  "url": "/api/data/v1/datasets/:dataset_id",
  "method": "DELETE",
  "name": "DeleteDataset",
  "reqType": "DeleteDatasetRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["dataset_id"]
  },
  "resType": "DeleteDatasetResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 获取数据集列表 */
export const ListDatasets = /*#__PURE__*/createAPI<ListDatasetsRequest, ListDatasetsResponse>({
  "url": "/api/data/v1/datasets/list",
  "method": "POST",
  "name": "ListDatasets",
  "reqType": "ListDatasetsRequest",
  "reqMapping": {
    "path": ["workspace_id"],
    "body": ["dataset_ids", "category", "name", "created_bys", "biz_categorys", "page_number", "page_size", "page_token", "order_bys"]
  },
  "resType": "ListDatasetsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 数据集当前信息（不包括数据） */
export const GetDataset = /*#__PURE__*/createAPI<GetDatasetRequest, GetDatasetResponse>({
  "url": "/api/data/v1/datasets/:dataset_id",
  "method": "GET",
  "name": "GetDataset",
  "reqType": "GetDatasetRequest",
  "reqMapping": {
    "query": ["workspace_id", "with_deleted"],
    "path": ["dataset_id"]
  },
  "resType": "GetDatasetResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量获取数据集 */
export const BatchGetDatasets = /*#__PURE__*/createAPI<BatchGetDatasetsRequest, BatchGetDatasetsResponse>({
  "url": "/api/data/v1/datasets/batch_get",
  "method": "POST",
  "name": "BatchGetDatasets",
  "reqType": "BatchGetDatasetsRequest",
  "reqMapping": {
    "body": ["workspace_id", "dataset_ids", "with_deleted"]
  },
  "resType": "BatchGetDatasetsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 导入数据 */
export const ImportDataset = /*#__PURE__*/createAPI<ImportDatasetRequest, ImportDatasetResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/import",
  "method": "POST",
  "name": "ImportDataset",
  "reqType": "ImportDatasetRequest",
  "reqMapping": {
    "body": ["workspace_id", "file", "field_mappings", "option"],
    "path": ["dataset_id"]
  },
  "resType": "ImportDatasetResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 任务(导入、导出、转换)详情 */
export const GetDatasetIOJob = /*#__PURE__*/createAPI<GetDatasetIOJobRequest, GetDatasetIOJobResponse>({
  "url": "/api/data/v1/dataset_io_jobs/:job_id",
  "method": "GET",
  "name": "GetDatasetIOJob",
  "reqType": "GetDatasetIOJobRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["job_id"]
  },
  "resType": "GetDatasetIOJobResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 数据集任务列表 */
export const ListDatasetIOJobs = /*#__PURE__*/createAPI<ListDatasetIOJobsRequest, ListDatasetIOJobsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/io_jobs",
  "method": "POST",
  "name": "ListDatasetIOJobs",
  "reqType": "ListDatasetIOJobsRequest",
  "reqMapping": {
    "body": ["workspace_id", "types", "statuses"],
    "path": ["dataset_id"]
  },
  "resType": "ListDatasetIOJobsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/**
 * Dataset Version
 * 生成一个新版本
*/
export const CreateDatasetVersion = /*#__PURE__*/createAPI<CreateDatasetVersionRequest, CreateDatasetVersionResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/versions",
  "method": "POST",
  "name": "CreateDatasetVersion",
  "reqType": "CreateDatasetVersionRequest",
  "reqMapping": {
    "body": ["workspace_id", "version", "desc"],
    "path": ["dataset_id"]
  },
  "resType": "CreateDatasetVersionResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 版本列表 */
export const ListDatasetVersions = /*#__PURE__*/createAPI<ListDatasetVersionsRequest, ListDatasetVersionsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/versions/list",
  "method": "POST",
  "name": "ListDatasetVersions",
  "reqType": "ListDatasetVersionsRequest",
  "reqMapping": {
    "body": ["workspace_id", "version_like", "page_number", "page_size", "page_token", "order_bys"],
    "path": ["dataset_id"]
  },
  "resType": "ListDatasetVersionsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 获取指定版本的数据集详情 */
export const GetDatasetVersion = /*#__PURE__*/createAPI<GetDatasetVersionRequest, GetDatasetVersionResponse>({
  "url": "/api/data/v1/dataset_versions/:version_id",
  "method": "GET",
  "name": "GetDatasetVersion",
  "reqType": "GetDatasetVersionRequest",
  "reqMapping": {
    "query": ["workspace_id", "with_deleted"],
    "path": ["version_id"]
  },
  "resType": "GetDatasetVersionResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量获取指定版本的数据集详情 */
export const BatchGetDatasetVersions = /*#__PURE__*/createAPI<BatchGetDatasetVersionsRequest, BatchGetDatasetVersionsResponse>({
  "url": "/api/data/v1/dataset_versions/batch_get",
  "method": "POST",
  "name": "BatchGetDatasetVersions",
  "reqType": "BatchGetDatasetVersionsRequest",
  "reqMapping": {
    "path": ["workspace_id"],
    "body": ["version_ids", "with_deleted"]
  },
  "resType": "BatchGetDatasetVersionsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/**
 * Dataset Schema
 * 获取数据集当前的 schema
*/
export const GetDatasetSchema = /*#__PURE__*/createAPI<GetDatasetSchemaRequest, GetDatasetSchemaResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/schema",
  "method": "GET",
  "name": "GetDatasetSchema",
  "reqType": "GetDatasetSchemaRequest",
  "reqMapping": {
    "query": ["workspace_id", "with_deleted"],
    "path": ["dataset_id"]
  },
  "resType": "GetDatasetSchemaResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 覆盖更新 schema */
export const UpdateDatasetSchema = /*#__PURE__*/createAPI<UpdateDatasetSchemaRequest, UpdateDatasetSchemaResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/schema",
  "method": "PUT",
  "name": "UpdateDatasetSchema",
  "reqType": "UpdateDatasetSchemaRequest",
  "reqMapping": {
    "body": ["workspace_id", "fields"],
    "path": ["dataset_id"]
  },
  "resType": "UpdateDatasetSchemaResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/**
 * Dataset Item
 * 校验数据
*/
export const ValidateDatasetItems = /*#__PURE__*/createAPI<ValidateDatasetItemsReq, ValidateDatasetItemsResp>({
  "url": "/api/data/v1/dataset_items/validate",
  "method": "POST",
  "name": "ValidateDatasetItems",
  "reqType": "ValidateDatasetItemsReq",
  "reqMapping": {
    "body": ["workspace_id", "items", "dataset_category", "dataset_fields", "ignore_current_item_count"],
    "path": ["dataset_id"]
  },
  "resType": "ValidateDatasetItemsResp",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量新增数据 */
export const BatchCreateDatasetItems = /*#__PURE__*/createAPI<BatchCreateDatasetItemsRequest, BatchCreateDatasetItemsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/batch_create",
  "method": "POST",
  "name": "BatchCreateDatasetItems",
  "reqType": "BatchCreateDatasetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "items", "skip_invalid_items", "allow_partial_add"],
    "path": ["dataset_id"]
  },
  "resType": "BatchCreateDatasetItemsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 更新数据 */
export const UpdateDatasetItem = /*#__PURE__*/createAPI<UpdateDatasetItemRequest, UpdateDatasetItemResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/:item_id",
  "method": "PUT",
  "name": "UpdateDatasetItem",
  "reqType": "UpdateDatasetItemRequest",
  "reqMapping": {
    "body": ["workspace_id", "data", "repeated_data"],
    "path": ["dataset_id", "item_id"]
  },
  "resType": "UpdateDatasetItemResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 删除数据 */
export const DeleteDatasetItem = /*#__PURE__*/createAPI<DeleteDatasetItemRequest, DeleteDatasetItemResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/:item_id",
  "method": "DELETE",
  "name": "DeleteDatasetItem",
  "reqType": "DeleteDatasetItemRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["dataset_id", "item_id"]
  },
  "resType": "DeleteDatasetItemResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量删除数据 */
export const BatchDeleteDatasetItems = /*#__PURE__*/createAPI<BatchDeleteDatasetItemsRequest, BatchDeleteDatasetItemsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/batch_delete",
  "method": "POST",
  "name": "BatchDeleteDatasetItems",
  "reqType": "BatchDeleteDatasetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "item_ids"],
    "path": ["dataset_id"]
  },
  "resType": "BatchDeleteDatasetItemsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 分页查询当前数据 */
export const ListDatasetItems = /*#__PURE__*/createAPI<ListDatasetItemsRequest, ListDatasetItemsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/list",
  "method": "POST",
  "name": "ListDatasetItems",
  "reqType": "ListDatasetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "page_token", "order_bys"],
    "path": ["dataset_id"]
  },
  "resType": "ListDatasetItemsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 分页查询指定版本的数据 */
export const ListDatasetItemsByVersion = /*#__PURE__*/createAPI<ListDatasetItemsByVersionRequest, ListDatasetItemsByVersionResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/versions/:version_id/items/list",
  "method": "POST",
  "name": "ListDatasetItemsByVersion",
  "reqType": "ListDatasetItemsByVersionRequest",
  "reqMapping": {
    "body": ["workspace_id", "page_number", "page_size", "page_token", "order_bys"],
    "path": ["dataset_id", "version_id"]
  },
  "resType": "ListDatasetItemsByVersionResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 获取一行数据 */
export const GetDatasetItem = /*#__PURE__*/createAPI<GetDatasetItemRequest, GetDatasetItemResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/:item_id",
  "method": "GET",
  "name": "GetDatasetItem",
  "reqType": "GetDatasetItemRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["dataset_id", "item_id"]
  },
  "resType": "GetDatasetItemResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量获取数据 */
export const BatchGetDatasetItems = /*#__PURE__*/createAPI<BatchGetDatasetItemsRequest, BatchGetDatasetItemsResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/batch_get",
  "method": "POST",
  "name": "BatchGetDatasetItems",
  "reqType": "BatchGetDatasetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "item_ids"],
    "path": ["dataset_id"]
  },
  "resType": "BatchGetDatasetItemsResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 批量获取指定版本的数据 */
export const BatchGetDatasetItemsByVersion = /*#__PURE__*/createAPI<BatchGetDatasetItemsByVersionRequest, BatchGetDatasetItemsByVersionResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/versions/:version_id/items/batch_get",
  "method": "POST",
  "name": "BatchGetDatasetItemsByVersion",
  "reqType": "BatchGetDatasetItemsByVersionRequest",
  "reqMapping": {
    "body": ["workspace_id", "item_ids"],
    "path": ["dataset_id", "version_id"]
  },
  "resType": "BatchGetDatasetItemsByVersionResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});
/** 清除(草稿)数据项 */
export const ClearDatasetItem = /*#__PURE__*/createAPI<ClearDatasetItemRequest, ClearDatasetItemResponse>({
  "url": "/api/data/v1/datasets/:dataset_id/items/clear",
  "method": "POST",
  "name": "ClearDatasetItem",
  "reqType": "ClearDatasetItemRequest",
  "reqMapping": {
    "body": ["workspace_id"],
    "path": ["dataset_id"]
  },
  "resType": "ClearDatasetItemResponse",
  "schemaRoot": "api://schemas/data_coze.loop.data.dataset",
  "service": "dataDataset"
});