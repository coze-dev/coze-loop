import * as dataset from './../data/domain/dataset';
export { dataset };
import * as common from './domain/common';
export { common };
import * as eval_set from './domain/eval_set';
export { eval_set };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateEvaluationSetRequest {
  workspace_id: string,
  name?: string,
  description?: string,
  evaluation_set_schema?: eval_set.EvaluationSetSchema,
  /** 业务分类 */
  biz_category?: eval_set.BizCategory,
  session?: common.Session,
}
export interface CreateEvaluationSetResponse {
  evaluation_set_id?: string
}
export interface UpdateEvaluationSetRequest {
  workspace_id: string,
  evaluation_set_id: string,
  name?: string,
  description?: string,
}
export interface UpdateEvaluationSetResponse {}
export interface DeleteEvaluationSetRequest {
  workspace_id: string,
  evaluation_set_id: string,
}
export interface DeleteEvaluationSetResponse {}
export interface GetEvaluationSetRequest {
  workspace_id: string,
  evaluation_set_id: string,
  deleted_at?: boolean,
}
export interface GetEvaluationSetResponse {
  evaluation_set?: eval_set.EvaluationSet
}
export interface ListEvaluationSetsRequest {
  workspace_id: string,
  /** 支持模糊搜索 */
  name?: string,
  creators?: string[],
  evaluation_set_ids?: string[],
  page_number?: number,
  /** 分页大小 (0, 200]，默认为 20 */
  page_size?: number,
  page_token?: string,
  /** 排列顺序，默认按照 createdAt 顺序排列，目前仅支持按照 createdAt 和 UpdatedAt 排序 */
  order_bys?: common.OrderBy[],
}
export interface ListEvaluationSetsResponse {
  evaluation_sets?: eval_set.EvaluationSet[],
  total?: string,
  next_page_token?: string,
}
export interface CreateEvaluationSetVersionRequest {
  workspace_id: string,
  evaluation_set_id: string,
  /** 展示的版本号，SemVer2 三段式，需要大于上一版本 */
  version?: string,
  desc?: string,
}
export interface CreateEvaluationSetVersionResponse {
  id?: string
}
export interface GetEvaluationSetVersionRequest {
  workspace_id: string,
  version_id: string,
  evaluation_set_id?: string,
  deleted_at?: boolean,
}
export interface GetEvaluationSetVersionResponse {
  version?: eval_set.EvaluationSetVersion,
  evaluation_set?: eval_set.EvaluationSet,
}
export interface BatchGetEvaluationSetVersionsRequest {
  workspace_id: string,
  version_ids: string[],
  deleted_at?: boolean,
}
export interface BatchGetEvaluationSetVersionsResponse {
  versioned_evaluation_sets?: VersionedEvaluationSet[]
}
export interface VersionedEvaluationSet {
  version?: eval_set.EvaluationSetVersion,
  evaluation_set?: eval_set.EvaluationSet,
}
export interface ListEvaluationSetVersionsRequest {
  workspace_id: string,
  evaluation_set_id: string,
  /** 根据版本号模糊匹配 */
  version_like?: string,
  page_number?: number,
  /** 分页大小 (0, 200]，默认为 20 */
  page_size?: number,
  page_token?: string,
}
export interface ListEvaluationSetVersionsResponse {
  versions?: eval_set.EvaluationSetVersion[],
  total?: string,
  next_page_token?: string,
}
export interface UpdateEvaluationSetSchemaRequest {
  workspace_id: string,
  evaluation_set_id: string,
  /**
   * fieldSchema.key 为空时：插入新的一列
   * fieldSchema.key 不为空时：更新对应的列
   * 硬删除（不支持恢复数据）的情况下，不需要写入入参的 field list；
   * 软删（支持恢复数据）的情况下，入参的 field list 中仍需保留该字段，并且需要把该字段的 deleted 置为 true
  */
  fields?: eval_set.FieldSchema[],
}
export interface UpdateEvaluationSetSchemaResponse {}
export interface BatchCreateEvaluationSetItemsRequest {
  workspace_id: string,
  evaluation_set_id: string,
  items?: eval_set.EvaluationSetItem[],
  /** items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据                                                    // items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据 */
  skip_invalid_items?: boolean,
  /** 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条 */
  allow_partial_add?: boolean,
}
export interface BatchCreateEvaluationSetItemsResponse {
  /** key: item 在 items 中的索引 */
  added_items?: {
    [key: string | number]: string
  },
  errors?: dataset.ItemErrorGroup[],
  item_outputs?: dataset.CreateDatasetItemOutput[],
}
export interface UpdateEvaluationSetItemRequest {
  workspace_id: string,
  evaluation_set_id: string,
  item_id: string,
  /** 每轮对话 */
  turns?: eval_set.Turn[],
}
export interface UpdateEvaluationSetItemResponse {}
export interface DeleteEvaluationSetItemRequest {
  workspace_id: string,
  evaluation_set_id: string,
  item_id: string,
}
export interface DeleteEvaluationSetItemResponse {}
export interface BatchDeleteEvaluationSetItemsRequest {
  workspace_id: string,
  evaluation_set_id: string,
  item_ids?: string[],
}
export interface BatchDeleteEvaluationSetItemsResponse {}
export interface ListEvaluationSetItemsRequest {
  workspace_id: string,
  evaluation_set_id: string,
  version_id?: string,
  page_number?: number,
  /** 分页大小 (0, 200]，默认为 20 */
  page_size?: number,
  page_token?: string,
  order_bys?: common.OrderBy[],
  item_id_not_in?: string[],
}
export interface ListEvaluationSetItemsResponse {
  items?: eval_set.EvaluationSetItem[],
  total?: string,
  next_page_token?: string,
}
export interface GetEvaluationSetItemRequest {
  workspace_id: string,
  evaluation_set_id: string,
  item_id: string,
}
export interface GetEvaluationSetItemResponse {
  item?: eval_set.EvaluationSetItem
}
export interface BatchGetEvaluationSetItemsRequest {
  workspace_id: string,
  evaluation_set_id: string,
  version_id?: string,
  item_ids?: string[],
}
export interface BatchGetEvaluationSetItemsResponse {
  items?: eval_set.EvaluationSetItem[]
}
export interface ClearEvaluationSetDraftItemRequest {
  workspace_id: string,
  evaluation_set_id: string,
}
export interface ClearEvaluationSetDraftItemResponse {}
/** 基本信息管理 */
export const CreateEvaluationSet = /*#__PURE__*/createAPI<CreateEvaluationSetRequest, CreateEvaluationSetResponse>({
  "url": "/api/evaluation/v1/evaluation_sets",
  "method": "POST",
  "name": "CreateEvaluationSet",
  "reqType": "CreateEvaluationSetRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "description", "evaluation_set_schema", "biz_category", "session"]
  },
  "resType": "CreateEvaluationSetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const UpdateEvaluationSet = /*#__PURE__*/createAPI<UpdateEvaluationSetRequest, UpdateEvaluationSetResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id",
  "method": "PATCH",
  "name": "UpdateEvaluationSet",
  "reqType": "UpdateEvaluationSetRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "description"],
    "path": ["evaluation_set_id"]
  },
  "resType": "UpdateEvaluationSetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const DeleteEvaluationSet = /*#__PURE__*/createAPI<DeleteEvaluationSetRequest, DeleteEvaluationSetResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id",
  "method": "DELETE",
  "name": "DeleteEvaluationSet",
  "reqType": "DeleteEvaluationSetRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["evaluation_set_id"]
  },
  "resType": "DeleteEvaluationSetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const GetEvaluationSet = /*#__PURE__*/createAPI<GetEvaluationSetRequest, GetEvaluationSetResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id",
  "method": "GET",
  "name": "GetEvaluationSet",
  "reqType": "GetEvaluationSetRequest",
  "reqMapping": {
    "query": ["workspace_id", "deleted_at"],
    "path": ["evaluation_set_id"]
  },
  "resType": "GetEvaluationSetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const ListEvaluationSets = /*#__PURE__*/createAPI<ListEvaluationSetsRequest, ListEvaluationSetsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/list",
  "method": "POST",
  "name": "ListEvaluationSets",
  "reqType": "ListEvaluationSetsRequest",
  "reqMapping": {
    "body": ["workspace_id", "name", "creators", "evaluation_set_ids", "page_number", "page_size", "page_token", "order_bys"]
  },
  "resType": "ListEvaluationSetsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
/** 版本管理 */
export const CreateEvaluationSetVersion = /*#__PURE__*/createAPI<CreateEvaluationSetVersionRequest, CreateEvaluationSetVersionResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions",
  "method": "POST",
  "name": "CreateEvaluationSetVersion",
  "reqType": "CreateEvaluationSetVersionRequest",
  "reqMapping": {
    "body": ["workspace_id", "version", "desc"],
    "path": ["evaluation_set_id"]
  },
  "resType": "CreateEvaluationSetVersionResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const GetEvaluationSetVersion = /*#__PURE__*/createAPI<GetEvaluationSetVersionRequest, GetEvaluationSetVersionResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/:version_id",
  "method": "GET",
  "name": "GetEvaluationSetVersion",
  "reqType": "GetEvaluationSetVersionRequest",
  "reqMapping": {
    "query": ["workspace_id", "deleted_at"],
    "path": ["version_id", "evaluation_set_id"]
  },
  "resType": "GetEvaluationSetVersionResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const ListEvaluationSetVersions = /*#__PURE__*/createAPI<ListEvaluationSetVersionsRequest, ListEvaluationSetVersionsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/list",
  "method": "POST",
  "name": "ListEvaluationSetVersions",
  "reqType": "ListEvaluationSetVersionsRequest",
  "reqMapping": {
    "body": ["workspace_id", "version_like", "page_number", "page_size", "page_token"],
    "path": ["evaluation_set_id"]
  },
  "resType": "ListEvaluationSetVersionsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const BatchGetEvaluationSetVersions = /*#__PURE__*/createAPI<BatchGetEvaluationSetVersionsRequest, BatchGetEvaluationSetVersionsResponse>({
  "url": "/api/evaluation/v1/evaluation_set_versions/batch_get",
  "method": "POST",
  "name": "BatchGetEvaluationSetVersions",
  "reqType": "BatchGetEvaluationSetVersionsRequest",
  "reqMapping": {
    "body": ["workspace_id", "version_ids", "deleted_at"]
  },
  "resType": "BatchGetEvaluationSetVersionsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
/** 字段管理 */
export const UpdateEvaluationSetSchema = /*#__PURE__*/createAPI<UpdateEvaluationSetSchemaRequest, UpdateEvaluationSetSchemaResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/schema",
  "method": "PUT",
  "name": "UpdateEvaluationSetSchema",
  "reqType": "UpdateEvaluationSetSchemaRequest",
  "reqMapping": {
    "body": ["workspace_id", "fields"],
    "path": ["evaluation_set_id"]
  },
  "resType": "UpdateEvaluationSetSchemaResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
/** 数据管理 */
export const BatchCreateEvaluationSetItems = /*#__PURE__*/createAPI<BatchCreateEvaluationSetItemsRequest, BatchCreateEvaluationSetItemsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_create",
  "method": "POST",
  "name": "BatchCreateEvaluationSetItems",
  "reqType": "BatchCreateEvaluationSetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "items", "skip_invalid_items", "allow_partial_add"],
    "path": ["evaluation_set_id"]
  },
  "resType": "BatchCreateEvaluationSetItemsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const UpdateEvaluationSetItem = /*#__PURE__*/createAPI<UpdateEvaluationSetItemRequest, UpdateEvaluationSetItemResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/:item_id",
  "method": "PUT",
  "name": "UpdateEvaluationSetItem",
  "reqType": "UpdateEvaluationSetItemRequest",
  "reqMapping": {
    "body": ["workspace_id", "turns"],
    "path": ["evaluation_set_id", "item_id"]
  },
  "resType": "UpdateEvaluationSetItemResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const BatchDeleteEvaluationSetItems = /*#__PURE__*/createAPI<BatchDeleteEvaluationSetItemsRequest, BatchDeleteEvaluationSetItemsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_delete",
  "method": "POST",
  "name": "BatchDeleteEvaluationSetItems",
  "reqType": "BatchDeleteEvaluationSetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "item_ids"],
    "path": ["evaluation_set_id"]
  },
  "resType": "BatchDeleteEvaluationSetItemsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const ListEvaluationSetItems = /*#__PURE__*/createAPI<ListEvaluationSetItemsRequest, ListEvaluationSetItemsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/list",
  "method": "POST",
  "name": "ListEvaluationSetItems",
  "reqType": "ListEvaluationSetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "version_id", "page_number", "page_size", "page_token", "order_bys", "item_id_not_in"],
    "path": ["evaluation_set_id"]
  },
  "resType": "ListEvaluationSetItemsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const BatchGetEvaluationSetItems = /*#__PURE__*/createAPI<BatchGetEvaluationSetItemsRequest, BatchGetEvaluationSetItemsResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/batch_get",
  "method": "POST",
  "name": "BatchGetEvaluationSetItems",
  "reqType": "BatchGetEvaluationSetItemsRequest",
  "reqMapping": {
    "body": ["workspace_id", "version_id", "item_ids"],
    "path": ["evaluation_set_id"]
  },
  "resType": "BatchGetEvaluationSetItemsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});
export const ClearEvaluationSetDraftItem = /*#__PURE__*/createAPI<ClearEvaluationSetDraftItemRequest, ClearEvaluationSetDraftItemResponse>({
  "url": "/api/evaluation/v1/evaluation_sets/:evaluation_set_id/items/clear",
  "method": "POST",
  "name": "ClearEvaluationSetDraftItem",
  "reqType": "ClearEvaluationSetDraftItemRequest",
  "reqMapping": {
    "body": ["workspace_id"],
    "path": ["evaluation_set_id"]
  },
  "resType": "ClearEvaluationSetDraftItemResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_set",
  "service": "evaluationEvalSet"
});