namespace go coze.loop.evaluation.eval_target

include "../../../base.thrift"
include "domain/common.thrift"
include "./domain/eval_target.thrift"

struct CreateEvalTargetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional CreateEvalTargetParam param

    255: optional base.Base Base
}

struct CreateEvalTargetParam {
    1: optional string source_target_id
    2: optional string source_target_version
    3: optional eval_target.EvalTargetType eval_target_type
    4: optional eval_target.CozeBotInfoType bot_info_type
    5: optional string bot_publish_version // 如果是发布版本则需要填充这个字段
}

struct CreateEvalTargetResponse {
    1: optional i64 id (api.js_conv="true", go.tag = 'json:"id"')
    2: optional i64 version_id (api.js_conv="true", go.tag = 'json:"version_id"')

    255: base.BaseResp BaseResp
}

struct GetEvalTargetVersionRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional i64 eval_target_version_id (api.path ='eval_target_version_id', api.js_conv="true", go.tag = 'json:"eval_target_version_id"')

    255: optional base.Base Base
}

struct GetEvalTargetVersionResponse {
    1: optional eval_target.EvalTarget eval_target

    255: base.BaseResp BaseResp
}

struct BatchGetEvalTargetVersionsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional list<i64> eval_target_version_ids (api.js_conv="true", go.tag = 'json:"eval_target_version_ids"')
    3: optional bool need_source_info

    255: optional base.Base Base
}

struct BatchGetEvalTargetVersionsResponse {
    1: optional list<eval_target.EvalTarget> eval_targets

    255: base.BaseResp BaseResp
}

struct BatchGetEvalTargetsBySourceRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional list<string> source_target_ids
    3: optional eval_target.EvalTargetType eval_target_type
    4: optional bool need_source_info

    255: optional base.Base Base
}

struct BatchGetEvalTargetsBySourceResponse {
    1: optional list<eval_target.EvalTarget> eval_targets

    255: base.BaseResp BaseResp
}

struct ExecuteEvalTargetRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: required i64 eval_target_id (api.path ='eval_target_id', api.js_conv="true", go.tag = 'json:"eval_target_id"')
    3: required i64 eval_target_version_id (api.path ='eval_target_version_id', api.js_conv="true", go.tag = 'json:"eval_target_version_id"')
    4: required eval_target.EvalTargetInputData input_data
    5: optional i64 experiment_run_id (api.js_conv="true", go.tag = 'json:"experiment_run_id"')

    255: optional base.Base Base

}

struct ExecuteEvalTargetResponse {
    1: required eval_target.EvalTargetRecord eval_target_record

    255: base.BaseResp BaseResp
}

struct ListEvalTargetRecordRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: required i64 eval_target_id (api.js_conv="true", go.tag = 'json:"eval_target_id"')
    3: optional list<i64> experiment_run_ids (api.js_conv="true", go.tag = 'json:"experiment_run_ids"')

    255: optional base.Base Base
}

struct ListEvalTargetRecordResponse {
    1: required list<eval_target.EvalTargetRecord> eval_target_records

    255: base.BaseResp BaseResp
}

struct GetEvalTargetRecordRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: required i64 eval_target_record_id  (api.path = 'eval_target_record_id', api.js_conv="true", go.tag = 'json:"eval_target_record_id"')

    255: optional base.Base Base
}

struct GetEvalTargetRecordResponse {
    1: optional eval_target.EvalTargetRecord eval_target_record

    255: base.BaseResp BaseResp
}

struct BatchGetEvalTargetRecordsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional list<i64> eval_target_record_ids (api.js_conv="true", go.tag = 'json:"eval_target_record_ids"')

    255: optional base.Base Base
}

struct BatchGetEvalTargetRecordsResponse {
    1: required list<eval_target.EvalTargetRecord> eval_target_records

    255: base.BaseResp BaseResp
}

struct ListSourceEvalTargetsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional eval_target.EvalTargetType target_type
    3: optional string name (vt.min_size = "1")   // 用户模糊搜索bot名称、promptkey

    100: optional i32 page_size
    101: optional string page_token

    255: optional base.Base Base
}

struct ListSourceEvalTargetsResponse {
    1: optional list<eval_target.EvalTarget> eval_targets

    100: optional string next_page_token
    101: optional bool has_more

    255: base.BaseResp BaseResp
}

struct BatchGetSourceEvalTargetsRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional list<string> source_target_ids
    3: optional eval_target.EvalTargetType target_type

    255: optional base.Base Base
}

struct BatchGetSourceEvalTargetsResponse {
    1: optional list<eval_target.EvalTarget> eval_targets

    255: base.BaseResp BaseResp
}

struct ListSourceEvalTargetVersionsRequest {
    1: required i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    2: required string source_target_id
    3: optional eval_target.EvalTargetType target_type

    100: optional i32 page_size
    101: optional string page_token

    255: optional base.Base Base
}

struct ListSourceEvalTargetVersionsResponse {
    1: optional list<eval_target.EvalTargetVersion> versions

    100: optional string next_page_token
    101: optional bool has_more

    255: base.BaseResp BaseResp
}

service EvalTargetService {
    // 创建评测对象
    CreateEvalTargetResponse CreateEvalTarget(1: CreateEvalTargetRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets")
    // 根据source target获取评测对象信息
    BatchGetEvalTargetsBySourceResponse BatchGetEvalTargetsBySource(1: BatchGetEvalTargetsBySourceRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/batch_get_by_source")
    // 获取评测对象+版本
    GetEvalTargetVersionResponse GetEvalTargetVersion(1: GetEvalTargetVersionRequest request) (api.category="eval_target", api.get = "/api/evaluation/v1/eval_target_versions/:eval_target_version_id")
    // 批量获取+版本
    BatchGetEvalTargetVersionsResponse BatchGetEvalTargetVersions(1: BatchGetEvalTargetVersionsRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_target_versions/batch_get")
    // Source评测对象列表
    ListSourceEvalTargetsResponse ListSourceEvalTargets(1: ListSourceEvalTargetsRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/list_source")
    // Source评测对象版本列表
    ListSourceEvalTargetVersionsResponse ListSourceEvalTargetVersions(1: ListSourceEvalTargetVersionsRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/list_source_version")
    BatchGetSourceEvalTargetsResponse BatchGetSourceEvalTargets (1: BatchGetSourceEvalTargetsRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/batch_get_source")
    // 执行
    ExecuteEvalTargetResponse ExecuteEvalTarget(1: ExecuteEvalTargetRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/:eval_target_id/versions/:eval_target_version_id/execute")
    GetEvalTargetRecordResponse GetEvalTargetRecord(1: GetEvalTargetRecordRequest request) (api.category="eval_target", api.get = "/api/evaluation/v1/eval_target_records/:eval_target_record_id")
    BatchGetEvalTargetRecordsResponse BatchGetEvalTargetRecords(1: BatchGetEvalTargetRecordsRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_target_records/batch_get")

} (api.js_conv="true" )
