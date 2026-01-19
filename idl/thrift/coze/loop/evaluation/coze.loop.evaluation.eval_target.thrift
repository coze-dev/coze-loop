namespace go coze.loop.evaluation.eval_target

include "../../../base.thrift"
include "domain/common.thrift"
include "./domain/eval_target.thrift"
include "coze.loop.evaluation.spi.thrift"

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
    6: optional eval_target.CustomEvalTarget custom_eval_target // type=6,并且有搜索对象，搜索结果信息通过这个字段透传
    7: optional eval_target.Region region   // 有区域限制需要填充这个字段
    8: optional string env  // 有环境限制需要填充这个字段
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

    10: optional eval_target.EvalTarget eval_target

    255: optional base.Base Base

}

struct ExecuteEvalTargetResponse {
    1: required eval_target.EvalTargetRecord eval_target_record

    255: base.BaseResp BaseResp
}

typedef ExecuteEvalTargetRequest AsyncExecuteEvalTargetRequest

struct AsyncExecuteEvalTargetResponse {
    1: optional i64 invoke_id
    2: optional string callee

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

struct SearchCustomEvalTargetRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"') // 空间ID
    2: optional string keyword // 透传spi接口
    3: optional i64 application_id (api.js_conv="true", go.tag = 'json:"application_id"') // 应用ID，非必填，创建实验时传应用ID,会根据应用ID从应用模块获取自定义服务详情
    4: optional eval_target.CustomRPCServer custom_rpc_server,    // 自定义服务详情，非必填，应用注册调试时传
    5: optional eval_target.Region region    // 必填
    6: optional string env    // 环境

    100: optional i32 page_size
    101: optional string page_token

    255: optional base.Base Base
}

struct SearchCustomEvalTargetResponse {
    1: list<eval_target.CustomEvalTarget> custom_eval_targets

    100: optional string next_page_token
    101: optional bool has_more

    255: base.BaseResp BaseResp (api.none="true")
}

struct DebugEvalTargetRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional eval_target.EvalTargetType eval_target_type    // 类型

    10: optional string param    // 执行参数：如果type=6,则传spi request json序列化结果
    11: optional common.RuntimeParam target_runtime_param    // 动态参数
    12: optional string env    // 环境

    50: optional eval_target.CustomRPCServer custom_rpc_server    // 如果type=6,需要前端传入自定义服务相关信息

    255: optional base.Base Base
}

struct DebugEvalTargetResponse {
    1: optional eval_target.EvalTargetRecord eval_target_record

    255: base.BaseResp BaseResp
}

struct AsyncDebugEvalTargetRequest {
    1: optional i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: optional eval_target.EvalTargetType eval_target_type    // 类型

    10: optional string param    // 执行参数：如果type=6,则传spi request json序列化结果
    11: optional common.RuntimeParam target_runtime_param    // 动态参数
    12: optional string env    // 环境

    50: optional eval_target.CustomRPCServer custom_rpc_server    // 如果type=6,需要前端传入自定义服务相关信息
    255: optional base.Base Base
}

struct MockEvalTargetOutputRequest {
    1: required i64 workspace_id (api.js_conv="true", go.tag = 'json:"workspace_id"')
    2: required i64 source_target_id (api.js_conv="true", go.tag = 'json:"source_target_id"') // EvalTargetID参数实际上为SourceTargetID
    3: required string eval_target_version
    4: required eval_target.EvalTargetType target_type

    255: optional base.Base Base
}

struct AsyncDebugEvalTargetResponse {
    1: required i64 invoke_id (api.js_conv="true", go.tag = 'json:"invoke_id"')
    2: optional string callee
    255: base.BaseResp BaseResp
}
struct MockEvalTargetOutputResponse {
    1: optional eval_target.EvalTarget eval_target
    2: optional map<string,string> mock_output

    255: base.BaseResp BaseResp
}

service EvalTargetService {
    // 创建评测对象
    CreateEvalTargetResponse CreateEvalTarget(1: CreateEvalTargetRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets", api.op_type = 'create', api.tag = 'volc-agentkit'
    )
    // 根据source target获取评测对象信息
    BatchGetEvalTargetsBySourceResponse BatchGetEvalTargetsBySource(1: BatchGetEvalTargetsBySourceRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/batch_get_by_source", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    // 获取评测对象+版本
    GetEvalTargetVersionResponse GetEvalTargetVersion(1: GetEvalTargetVersionRequest request) (
        api.category="eval_target", api.get = "/api/evaluation/v1/eval_target_versions/:eval_target_version_id", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    // 批量获取+版本
    BatchGetEvalTargetVersionsResponse BatchGetEvalTargetVersions(1: BatchGetEvalTargetVersionsRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_target_versions/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    // Source评测对象列表
    ListSourceEvalTargetsResponse ListSourceEvalTargets(1: ListSourceEvalTargetsRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/list_source", api.op_type = 'list', api.tag = 'volc-agentkit'
    )
    // Source评测对象版本列表
    ListSourceEvalTargetVersionsResponse ListSourceEvalTargetVersions(1: ListSourceEvalTargetVersionsRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/list_source_version", api.op_type = 'list', api.tag = 'volc-agentkit'
    )
    BatchGetSourceEvalTargetsResponse BatchGetSourceEvalTargets (1: BatchGetSourceEvalTargetsRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/batch_get_source", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    // 搜索自定义评测对象
    SearchCustomEvalTargetResponse SearchCustomEvalTarget(1: SearchCustomEvalTargetRequest req) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/search_custom")

    // 执行
    ExecuteEvalTargetResponse ExecuteEvalTarget(1: ExecuteEvalTargetRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/:eval_target_id/versions/:eval_target_version_id/execute")
    AsyncExecuteEvalTargetResponse AsyncExecuteEvalTarget(1: AsyncExecuteEvalTargetRequest request)
    GetEvalTargetRecordResponse GetEvalTargetRecord(1: GetEvalTargetRecordRequest request) (
        api.category="eval_target", api.get = "/api/evaluation/v1/eval_target_records/:eval_target_record_id", api.op_type = 'query', api.tag = 'volc-agentkit'
    )
    BatchGetEvalTargetRecordsResponse BatchGetEvalTargetRecords(1: BatchGetEvalTargetRecordsRequest request) (
        api.category="eval_target", api.post = "/api/evaluation/v1/eval_target_records/batch_get", api.op_type = 'query', api.tag = 'volc-agentkit'
    )

    // debug
    DebugEvalTargetResponse DebugEvalTarget(1: DebugEvalTargetRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/debug")
    AsyncDebugEvalTargetResponse AsyncDebugEvalTarget(1: AsyncDebugEvalTargetRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/async_debug")

    // mock输出数据
    MockEvalTargetOutputResponse MockEvalTargetOutput(1: MockEvalTargetOutputRequest request) (api.category="eval_target", api.post = "/api/evaluation/v1/eval_targets/mock_output")
}
