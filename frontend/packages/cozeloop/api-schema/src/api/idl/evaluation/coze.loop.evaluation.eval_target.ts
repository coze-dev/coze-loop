import * as coze_loop_evaluation_spi from './coze.loop.evaluation.spi';
export { coze_loop_evaluation_spi };
import * as eval_target from './domain/eval_target';
export { eval_target };
import * as common from './domain/common';
export { common };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface CreateEvalTargetRequest {
  workspace_id: string,
  param?: CreateEvalTargetParam,
}
export interface CreateEvalTargetParam {
  source_target_id?: string,
  source_target_version?: string,
  eval_target_type?: eval_target.EvalTargetType,
  bot_info_type?: eval_target.CozeBotInfoType,
  /** 如果是发布版本则需要填充这个字段 */
  bot_publish_version?: string,
  /** type=6,并且有搜索对象，搜索结果信息通过这个字段透传 */
  custom_eval_target?: eval_target.CustomEvalTarget,
  /** 有区域限制需要填充这个字段 */
  region?: eval_target.Region,
  /** 有环境限制需要填充这个字段 */
  env?: string,
}
export interface CreateEvalTargetResponse {
  id?: string,
  version_id?: string,
}
export interface GetEvalTargetVersionRequest {
  workspace_id: string,
  eval_target_version_id?: string,
}
export interface GetEvalTargetVersionResponse {
  eval_target?: eval_target.EvalTarget
}
export interface BatchGetEvalTargetVersionsRequest {
  workspace_id: string,
  eval_target_version_ids?: string[],
  need_source_info?: boolean,
}
export interface BatchGetEvalTargetVersionsResponse {
  eval_targets?: eval_target.EvalTarget[]
}
export interface BatchGetEvalTargetsBySourceRequest {
  workspace_id: string,
  source_target_ids?: string[],
  eval_target_type?: eval_target.EvalTargetType,
  need_source_info?: boolean,
}
export interface BatchGetEvalTargetsBySourceResponse {
  eval_targets?: eval_target.EvalTarget[]
}
export interface ExecuteEvalTargetRequest {
  workspace_id: string,
  eval_target_id: string,
  eval_target_version_id: string,
  input_data: eval_target.EvalTargetInputData,
  experiment_run_id?: string,
  eval_target?: eval_target.EvalTarget,
}
export interface ExecuteEvalTargetResponse {
  eval_target_record: eval_target.EvalTargetRecord
}
export type AsyncExecuteEvalTargetRequest = ExecuteEvalTargetRequest;
export interface AsyncExecuteEvalTargetResponse {
  invoke_id?: number,
  callee?: string,
}
export interface ListEvalTargetRecordRequest {
  workspace_id: string,
  eval_target_id: string,
  experiment_run_ids?: string[],
}
export interface ListEvalTargetRecordResponse {
  eval_target_records: eval_target.EvalTargetRecord[]
}
export interface GetEvalTargetRecordRequest {
  workspace_id: string,
  eval_target_record_id: string,
}
export interface GetEvalTargetRecordResponse {
  eval_target_record?: eval_target.EvalTargetRecord
}
export interface BatchGetEvalTargetRecordsRequest {
  workspace_id: string,
  eval_target_record_ids?: string[],
}
export interface BatchGetEvalTargetRecordsResponse {
  eval_target_records: eval_target.EvalTargetRecord[]
}
export interface ListSourceEvalTargetsRequest {
  workspace_id: string,
  target_type?: eval_target.EvalTargetType,
  /** 用户模糊搜索bot名称、promptkey */
  name?: string,
  page_size?: number,
  page_token?: string,
}
export interface ListSourceEvalTargetsResponse {
  eval_targets?: eval_target.EvalTarget[],
  next_page_token?: string,
  has_more?: boolean,
}
export interface BatchGetSourceEvalTargetsRequest {
  workspace_id: string,
  source_target_ids?: string[],
  target_type?: eval_target.EvalTargetType,
}
export interface BatchGetSourceEvalTargetsResponse {
  eval_targets?: eval_target.EvalTarget[]
}
export interface ListSourceEvalTargetVersionsRequest {
  workspace_id: string,
  source_target_id: string,
  target_type?: eval_target.EvalTargetType,
  page_size?: number,
  page_token?: string,
}
export interface ListSourceEvalTargetVersionsResponse {
  versions?: eval_target.EvalTargetVersion[],
  next_page_token?: string,
  has_more?: boolean,
}
export interface SearchCustomEvalTargetRequest {
  /** 空间ID */
  workspace_id?: string,
  /** 透传spi接口 */
  keyword?: string,
  /** 应用ID，非必填，创建实验时传应用ID,会根据应用ID从应用模块获取自定义服务详情 */
  application_id?: string,
  /** 自定义服务详情，非必填，应用注册调试时传 */
  custom_rpc_server?: eval_target.CustomRPCServer,
  /** 必填 */
  region?: eval_target.Region,
  /** 环境 */
  env?: string,
  page_size?: number,
  page_token?: string,
}
export interface SearchCustomEvalTargetResponse {
  custom_eval_targets: eval_target.CustomEvalTarget[],
  next_page_token?: string,
  has_more?: boolean,
}
export interface DebugEvalTargetRequest {
  workspace_id?: string,
  /** 类型 */
  eval_target_type?: eval_target.EvalTargetType,
  /** 执行参数：如果type=6,则传spi request json序列化结果 */
  param?: string,
  /** 动态参数 */
  target_runtime_param?: common.RuntimeParam,
  /** 环境 */
  env?: string,
  /** 如果type=6,需要前端传入自定义服务相关信息 */
  custom_rpc_server?: eval_target.CustomRPCServer,
}
export interface DebugEvalTargetResponse {
  eval_target_record?: eval_target.EvalTargetRecord
}
export interface AsyncDebugEvalTargetRequest {
  workspace_id?: string,
  /** 类型 */
  eval_target_type?: eval_target.EvalTargetType,
  /** 执行参数：如果type=6,则传spi request json序列化结果 */
  param?: string,
  /** 动态参数 */
  target_runtime_param?: common.RuntimeParam,
  /** 环境 */
  env?: string,
  /** 如果type=6,需要前端传入自定义服务相关信息 */
  custom_rpc_server?: eval_target.CustomRPCServer,
}
export interface MockEvalTargetOutputRequest {
  workspace_id: string,
  /** EvalTargetID参数实际上为SourceTargetID */
  source_target_id: string,
  eval_target_version: string,
  target_type: eval_target.EvalTargetType,
}
export interface AsyncDebugEvalTargetResponse {
  invoke_id: string,
  callee?: string,
}
export interface MockEvalTargetOutputResponse {
  eval_target?: eval_target.EvalTarget,
  mock_output?: {
    [key: string | number]: string
  },
}
/** 创建评测对象 */
export const CreateEvalTarget = /*#__PURE__*/createAPI<CreateEvalTargetRequest, CreateEvalTargetResponse>({
  "url": "/api/evaluation/v1/eval_targets",
  "method": "POST",
  "name": "CreateEvalTarget",
  "reqType": "CreateEvalTargetRequest",
  "reqMapping": {
    "body": ["workspace_id", "param"]
  },
  "resType": "CreateEvalTargetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** 根据source target获取评测对象信息 */
export const BatchGetEvalTargetsBySource = /*#__PURE__*/createAPI<BatchGetEvalTargetsBySourceRequest, BatchGetEvalTargetsBySourceResponse>({
  "url": "/api/evaluation/v1/eval_targets/batch_get_by_source",
  "method": "POST",
  "name": "BatchGetEvalTargetsBySource",
  "reqType": "BatchGetEvalTargetsBySourceRequest",
  "reqMapping": {
    "body": ["workspace_id", "source_target_ids", "eval_target_type", "need_source_info"]
  },
  "resType": "BatchGetEvalTargetsBySourceResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** 获取评测对象+版本 */
export const GetEvalTargetVersion = /*#__PURE__*/createAPI<GetEvalTargetVersionRequest, GetEvalTargetVersionResponse>({
  "url": "/api/evaluation/v1/eval_target_versions/:eval_target_version_id",
  "method": "GET",
  "name": "GetEvalTargetVersion",
  "reqType": "GetEvalTargetVersionRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["eval_target_version_id"]
  },
  "resType": "GetEvalTargetVersionResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** 批量获取+版本 */
export const BatchGetEvalTargetVersions = /*#__PURE__*/createAPI<BatchGetEvalTargetVersionsRequest, BatchGetEvalTargetVersionsResponse>({
  "url": "/api/evaluation/v1/eval_target_versions/batch_get",
  "method": "POST",
  "name": "BatchGetEvalTargetVersions",
  "reqType": "BatchGetEvalTargetVersionsRequest",
  "reqMapping": {
    "body": ["workspace_id", "eval_target_version_ids", "need_source_info"]
  },
  "resType": "BatchGetEvalTargetVersionsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** Source评测对象列表 */
export const ListSourceEvalTargets = /*#__PURE__*/createAPI<ListSourceEvalTargetsRequest, ListSourceEvalTargetsResponse>({
  "url": "/api/evaluation/v1/eval_targets/list_source",
  "method": "POST",
  "name": "ListSourceEvalTargets",
  "reqType": "ListSourceEvalTargetsRequest",
  "reqMapping": {
    "body": ["workspace_id", "target_type", "name", "page_size", "page_token"]
  },
  "resType": "ListSourceEvalTargetsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** Source评测对象版本列表 */
export const ListSourceEvalTargetVersions = /*#__PURE__*/createAPI<ListSourceEvalTargetVersionsRequest, ListSourceEvalTargetVersionsResponse>({
  "url": "/api/evaluation/v1/eval_targets/list_source_version",
  "method": "POST",
  "name": "ListSourceEvalTargetVersions",
  "reqType": "ListSourceEvalTargetVersionsRequest",
  "reqMapping": {
    "body": ["workspace_id", "source_target_id", "target_type", "page_size", "page_token"]
  },
  "resType": "ListSourceEvalTargetVersionsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
export const BatchGetSourceEvalTargets = /*#__PURE__*/createAPI<BatchGetSourceEvalTargetsRequest, BatchGetSourceEvalTargetsResponse>({
  "url": "/api/evaluation/v1/eval_targets/batch_get_source",
  "method": "POST",
  "name": "BatchGetSourceEvalTargets",
  "reqType": "BatchGetSourceEvalTargetsRequest",
  "reqMapping": {
    "body": ["workspace_id", "source_target_ids", "target_type"]
  },
  "resType": "BatchGetSourceEvalTargetsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** 搜索自定义评测对象 */
export const SearchCustomEvalTarget = /*#__PURE__*/createAPI<SearchCustomEvalTargetRequest, SearchCustomEvalTargetResponse>({
  "url": "/api/evaluation/v1/eval_targets/search_custom",
  "method": "POST",
  "name": "SearchCustomEvalTarget",
  "reqType": "SearchCustomEvalTargetRequest",
  "reqMapping": {
    "body": ["workspace_id", "keyword", "application_id", "custom_rpc_server", "region", "env", "page_size", "page_token"]
  },
  "resType": "SearchCustomEvalTargetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** 执行 */
export const ExecuteEvalTarget = /*#__PURE__*/createAPI<ExecuteEvalTargetRequest, ExecuteEvalTargetResponse>({
  "url": "/api/evaluation/v1/eval_targets/:eval_target_id/versions/:eval_target_version_id/execute",
  "method": "POST",
  "name": "ExecuteEvalTarget",
  "reqType": "ExecuteEvalTargetRequest",
  "reqMapping": {
    "body": ["workspace_id", "input_data", "experiment_run_id", "eval_target"],
    "path": ["eval_target_id", "eval_target_version_id"]
  },
  "resType": "ExecuteEvalTargetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
export const GetEvalTargetRecord = /*#__PURE__*/createAPI<GetEvalTargetRecordRequest, GetEvalTargetRecordResponse>({
  "url": "/api/evaluation/v1/eval_target_records/:eval_target_record_id",
  "method": "GET",
  "name": "GetEvalTargetRecord",
  "reqType": "GetEvalTargetRecordRequest",
  "reqMapping": {
    "query": ["workspace_id"],
    "path": ["eval_target_record_id"]
  },
  "resType": "GetEvalTargetRecordResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
export const BatchGetEvalTargetRecords = /*#__PURE__*/createAPI<BatchGetEvalTargetRecordsRequest, BatchGetEvalTargetRecordsResponse>({
  "url": "/api/evaluation/v1/eval_target_records/batch_get",
  "method": "POST",
  "name": "BatchGetEvalTargetRecords",
  "reqType": "BatchGetEvalTargetRecordsRequest",
  "reqMapping": {
    "body": ["workspace_id", "eval_target_record_ids"]
  },
  "resType": "BatchGetEvalTargetRecordsResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** debug */
export const DebugEvalTarget = /*#__PURE__*/createAPI<DebugEvalTargetRequest, DebugEvalTargetResponse>({
  "url": "/api/evaluation/v1/eval_targets/debug",
  "method": "POST",
  "name": "DebugEvalTarget",
  "reqType": "DebugEvalTargetRequest",
  "reqMapping": {
    "body": ["workspace_id", "eval_target_type", "param", "target_runtime_param", "env", "custom_rpc_server"]
  },
  "resType": "DebugEvalTargetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
export const AsyncDebugEvalTarget = /*#__PURE__*/createAPI<AsyncDebugEvalTargetRequest, AsyncDebugEvalTargetResponse>({
  "url": "/api/evaluation/v1/eval_targets/async_debug",
  "method": "POST",
  "name": "AsyncDebugEvalTarget",
  "reqType": "AsyncDebugEvalTargetRequest",
  "reqMapping": {
    "body": ["workspace_id", "eval_target_type", "param", "target_runtime_param", "env", "custom_rpc_server"]
  },
  "resType": "AsyncDebugEvalTargetResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});
/** mock输出数据 */
export const MockEvalTargetOutput = /*#__PURE__*/createAPI<MockEvalTargetOutputRequest, MockEvalTargetOutputResponse>({
  "url": "/api/evaluation/v1/eval_targets/mock_output",
  "method": "POST",
  "name": "MockEvalTargetOutput",
  "reqType": "MockEvalTargetOutputRequest",
  "reqMapping": {
    "body": ["workspace_id", "source_target_id", "eval_target_version", "target_type"]
  },
  "resType": "MockEvalTargetOutputResponse",
  "schemaRoot": "api://schemas/evaluation_coze.loop.evaluation.eval_target",
  "service": "evaluationEvalTarget"
});