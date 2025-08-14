namespace go coze.loop.evaluation.evaluator

include "../../../base.thrift"
include "./domain/common.thrift"
include "./domain/evaluator.thrift"

struct ListEvaluatorsRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional string search_name (api.body='search_name')
    3: optional list<i64> creator_ids (api.body='creator_ids', api.js_conv='true', go.tag='json:"creator_ids"')
    4: optional list<evaluator.EvaluatorType> evaluator_type (api.body='evaluator_type')
    5: optional bool with_version (api.body='with_version')

    101: optional i32 page_size (api.body='page_size', vt.gt='0')
    102: optional i32 page_number (api.body='page_number', vt.gt='0')
    103: optional list<common.OrderBy> order_bys (api.body='order_bys')

    255: optional base.Base Base
}

struct ListEvaluatorsResponse {
    1: optional list<evaluator.Evaluator> evaluators (api.body='evaluators', go.tag='json:"evaluators"')
    10: optional i64 total (api.body='total', api.js_conv='true', go.tag='json:"total"')
    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorsRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_ids (api.body='evaluator_ids', api.js_conv='true', go.tag='json:"evaluator_ids"')
    3: optional bool include_deleted (api.body='include_deleted') // 是否查询已删除的评估器，默认不查询

    255: optional base.Base Base
}

struct BatchGetEvaluatorsResponse {
    1: optional list<evaluator.Evaluator> evaluators (api.body='evaluators')

    255: base.BaseResp BaseResp
}

struct GetEvaluatorRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')
    3: optional bool include_deleted (api.query='include_deleted') // 是否查询已删除的评估器，默认不查询

    255: optional base.Base Base
}

struct GetEvaluatorResponse {
    1: optional evaluator.Evaluator evaluator (api.body='evaluator')

    255: base.BaseResp BaseResp
}

struct CreateEvaluatorRequest {
    1: required evaluator.Evaluator evaluator (api.body='evaluator')
    100: optional string cid (api.body='cid')

    255: optional base.Base Base
}

struct CreateEvaluatorResponse {
    1: optional i64 evaluator_id (api.body='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')

    255: base.BaseResp BaseResp
}

struct UpdateEvaluatorDraftRequest {
    1: required i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')  // 评估器 id
    2: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')  // 空间 id
    3: required evaluator.EvaluatorContent evaluator_content (api.body='evaluator_content', go.tag='json:"evaluator_content"')
    4: required evaluator.EvaluatorType evaluator_type (api.body='evaluator_type', go.tag='json:"evaluator_type"')

    255: optional base.Base Base
}

struct UpdateEvaluatorDraftResponse {
    1: optional evaluator.Evaluator evaluator (api.body='evaluator')

    255: base.BaseResp BaseResp
}

struct UpdateEvaluatorRequest {
    1: required i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')  // 评估器 id
    2: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')  // 空间 id
    3: required evaluator.EvaluatorType evaluator_type (api.body='evaluator_type', go.tag='json:"evaluator_type"')
    4: optional string name (api.body='name', go.tag='json:"name"') // 展示用名称
    5: optional string description (api.body='description', go.tag='json:"description"') // 描述

    255: optional base.Base Base
}

struct UpdateEvaluatorResponse {
    255: base.BaseResp BaseResp
}

struct CloneEvaluatorRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')

    255: optional base.Base Base
}

struct CloneEvaluatorResponse {
    1: optional i64 evaluator_id (api.body='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')

    255: base.BaseResp BaseResp
}

struct ListEvaluatorVersionsRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')
    3: optional list<string> query_versions (api.body='query_versions')
    101: optional i32 page_size (api.body='page_size', vt.gt='0')
    102: optional i32 page_number (api.body='page_number', vt.gt='0')
    103: optional list<common.OrderBy> order_bys (api.body='order_bys')

    255: optional base.Base Base
}

struct ListEvaluatorVersionsResponse {
    1: optional list<evaluator.EvaluatorVersion> evaluator_versions (api.body='evaluator_versions')
    10: optional i64 total (api.body='total', api.js_conv='true', go.tag='json:"total"')

    255: base.BaseResp BaseResp
}

struct GetEvaluatorVersionRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_version_id (api.path='evaluator_version_id', api.js_conv='true', go.tag='json:"evaluator_version_id"')
    3: optional bool include_deleted (api.query='include_deleted') // 是否查询已删除的评估器，默认不查询

    255: optional base.Base Base
}

struct GetEvaluatorVersionResponse {
    1: optional evaluator.Evaluator evaluator (api.body='evaluator')

    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorVersionsRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_version_ids (api.body='evaluator_version_ids', api.js_conv='true', go.tag='json:"evaluator_version_ids"')
    3: optional bool include_deleted (api.body='include_deleted') // 是否查询已删除的评估器，默认不查询

    255: optional base.Base Base
}

struct BatchGetEvaluatorVersionsResponse {
    1: optional list<evaluator.Evaluator> evaluators  (api.body='evaluators')

    255: base.BaseResp BaseResp
}

struct SubmitEvaluatorVersionRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')
    3: required string version (api.body='version')
    4: optional string description (api.body='description')
    100: optional string cid (api.body='cid')

    255: optional base.Base Base
}

struct SubmitEvaluatorVersionResponse {
    1: optional evaluator.Evaluator evaluator  (api.body='evaluator')

    255: base.BaseResp BaseResp
}

struct ListTemplatesRequest {
    1: required evaluator.TemplateType builtin_template_type (api.query='builtin_template_type')

    255: optional base.Base Base
}

struct ListTemplatesResponse {
    1: optional list<evaluator.EvaluatorContent> builtin_template_keys  (api.body='builtin_template_keys')

    255: base.BaseResp BaseResp
}

struct GetTemplateInfoRequest {
    1: required evaluator.TemplateType builtin_template_type (api.query='builtin_template_type')
    2: required string builtin_template_key (api.query='builtin_template_key')

    255: optional base.Base Base
}

struct GetTemplateInfoResponse {
    1: optional evaluator.EvaluatorContent evaluator_content (api.body='builtin_template')

    255: base.BaseResp BaseResp
}

struct RunEvaluatorRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"') // 空间 id
    2: required i64 evaluator_version_id (api.path='evaluator_version_id', api.js_conv='true', go.tag='json:"evaluator_version_id"')                     // 评测规则 id
    3: required evaluator.EvaluatorInputData input_data (api.body='input_data')         // 评测数据输入: 数据集行内容 + 评测目标输出内容与历史记录 + 评测目标的 trace
    4: optional i64 experiment_id (api.body='experiment_id', api.js_conv='true', go.tag='json:"experiment_id"')                          // experiment id
    5: optional i64 experiment_run_id (api.body='experiment_run_id', api.js_conv='true', go.tag='json:"experiment_run_id"')                          // experiment run id
    6: optional i64 item_id (api.body='item_id', api.js_conv='true', go.tag='json:"item_id"')
    7: optional i64 turn_id (api.body='turn_id', api.js_conv='true', go.tag='json:"turn_id"')

    100: optional map<string, string> ext (api.body='ext')

    255: optional base.Base Base
}

struct RunEvaluatorResponse {
    1: required evaluator.EvaluatorRecord record (api.body='record')

    255: base.BaseResp BaseResp
}

struct DebugEvaluatorRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"') // 空间 id
    2: required evaluator.EvaluatorContent evaluator_content (api.body='evaluator_content')                     // 待调试评估器内容
    3: required evaluator.EvaluatorInputData input_data (api.body='input_data')         // 评测数据输入: 数据集行内容 + 评测目标输出内容与历史记录 + 评测目标的 trace
    4: required evaluator.EvaluatorType evaluator_type (api.body='evaluator_type', go.tag='json:"evaluator_type"')

    255: optional base.Base Base
}

struct DebugEvaluatorResponse {
    1: optional evaluator.EvaluatorOutputData evaluator_output_data (api.body='evaluator_output_data') // 输出数据

    255: base.BaseResp BaseResp
}

struct DeleteEvaluatorRequest {
    1: optional i64 evaluator_id (api.path='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')
    2: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')

    255: optional base.Base Base
}

struct DeleteEvaluatorResponse {
    255: base.BaseResp BaseResp
}

struct CheckEvaluatorNameRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required string name (api.body='name')
    3: optional i64 evaluator_id (api.body='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')

    255: optional base.Base Base
}

struct CheckEvaluatorNameResponse {
    1: optional bool pass (api.body='pass')
    2: optional string message (api.body='message')

    255: base.BaseResp BaseResp
}

struct ListEvaluatorRecordRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_id (api.body='evaluator_id', api.js_conv='true', go.tag='json:"evaluator_id"')
    3: optional list<i64> experiment_run_ids (api.body='experiment_run_ids', api.js_conv='true', go.tag='json:"experiment_run_ids"')
    101: optional i32 page_size (api.body='page_size', vt.gt='0', vt.le='200'),    // 分页大小 (0, 200]，默认为 20
    102: optional string page_token (api.body='page_token')

    255: optional base.Base Base
}

struct ListEvaluatorRecordResponse {
    1: required list<evaluator.EvaluatorRecord> records (api.body='records')

    255: base.BaseResp BaseResp
}

struct GetEvaluatorRecordRequest {
    1: required i64 workspace_id (api.query='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_record_id (api.path='evaluator_record_id', api.js_conv='true', go.tag='json:"evaluator_record_id"')
    3: optional bool include_deleted (api.query='include_deleted') // 是否查询已删除的，默认不查询

    255: optional base.Base Base
}

struct GetEvaluatorRecordResponse {
    1: required evaluator.EvaluatorRecord record (api.body='record')
    255: base.BaseResp BaseResp
}

struct BatchGetEvaluatorRecordsRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<i64> evaluator_record_ids (api.body='evaluator_record_ids', api.js_conv='true', go.tag='json:"evaluator_record_ids"')
    3: optional bool include_deleted (api.body='include_deleted') // 是否查询已删除的，默认不查询

    255: optional base.Base Base
}

struct BatchGetEvaluatorRecordsResponse {
    1: required list<evaluator.EvaluatorRecord> records (api.body='records')
    255: base.BaseResp BaseResp
}

struct UpdateEvaluatorRecordRequest {
    1: required i64 workspace_id (api.body='workspace_id', api.js_conv='true', go.tag='json:"workspace_id"')
    2: required i64 evaluator_record_id (api.path='evaluator_record_id', api.js_conv='true', go.tag='json:"evaluator_record_id"')
    3: required evaluator.Correction correction (api.body='correction')

    255: optional base.Base Base
}

struct UpdateEvaluatorRecordResponse {
    1: required evaluator.EvaluatorRecord record (api.body='record')
    255: base.BaseResp BaseResp
}

struct GetDefaultPromptEvaluatorToolsRequest {
    255: optional base.Base Base
}

struct GetDefaultPromptEvaluatorToolsResponse {
    1: required list<evaluator.Tool> tools (api.body='tools')

    255: base.BaseResp BaseResp
}

service EvaluatorService {
    // 评估器
    ListEvaluatorsResponse ListEvaluators(1: ListEvaluatorsRequest request)           (api.post=  "/api/evaluation/v1/evaluators/list")      // 按查询条件查询evaluator
    BatchGetEvaluatorsResponse BatchGetEvaluators(1: BatchGetEvaluatorsRequest request)           (api.post=  "/api/evaluation/v1/evaluators/batch_get")      // 按id批量查询evaluator
    GetEvaluatorResponse GetEvaluator(1: GetEvaluatorRequest request)           (api.get=  "/api/evaluation/v1/evaluators/:evaluator_id")      // 按id单个查询evaluator
    CreateEvaluatorResponse CreateEvaluator(1: CreateEvaluatorRequest request)     (api.post=  "/api/evaluation/v1/evaluators")           // 创建evaluator
    UpdateEvaluatorResponse UpdateEvaluator(1: UpdateEvaluatorRequest request)     (api.patch=   "/api/evaluation/v1/evaluators/:evaluator_id")  // 修改evaluator元信息
    UpdateEvaluatorDraftResponse UpdateEvaluatorDraft(1: UpdateEvaluatorDraftRequest request)     (api.patch=   "/api/evaluation/v1/evaluators/:evaluator_id/update_draft")  // 修改evaluator草稿
    DeleteEvaluatorResponse DeleteEvaluator(1: DeleteEvaluatorRequest request)     (api.delete=   "/api/evaluation/v1/evaluators/:evaluator_id")   // 批量删除evaluator
    CheckEvaluatorNameResponse CheckEvaluatorName(1: CheckEvaluatorNameRequest request)     (api.post=   "/api/evaluation/v1/evaluators/check_name")   // 校验evaluator名称是否重复

    // 评估器版本
    ListEvaluatorVersionsResponse ListEvaluatorVersions(1: ListEvaluatorVersionsRequest request)           (api.post=  "/api/evaluation/v1/evaluators/:evaluator_id/versions/list")      // 按evaluator id查询evaluator version
    GetEvaluatorVersionResponse GetEvaluatorVersion(1: GetEvaluatorVersionRequest request)           (api.get=  "/api/evaluation/v1/evaluators_versions/:evaluator_version_id")      // 按版本id单个查询evaluator version
    BatchGetEvaluatorVersionsResponse BatchGetEvaluatorVersions(1: BatchGetEvaluatorVersionsRequest request)           (api.post=  "/api/evaluation/v1/evaluators_versions/batch_get")      // 按版本id批量查询evaluator version
    SubmitEvaluatorVersionResponse SubmitEvaluatorVersion(1: SubmitEvaluatorVersionRequest request)     (api.post=   "/api/evaluation/v1/evaluators/:evaluator_id/submit_version")   // 提交evaluator版本

    // 评估器预置模版
    ListTemplatesResponse ListTemplates(1: ListTemplatesRequest request)           (api.post=  "/api/evaluation/v1/evaluators/list_template")      // 获取内置评估器模板列表（不含具体内容）
    GetTemplateInfoResponse GetTemplateInfo(1: GetTemplateInfoRequest request) (api.post=  "/api/evaluation/v1/evaluators/get_template_info")      // 按key单个查询内置评估器模板详情
    GetDefaultPromptEvaluatorToolsResponse GetDefaultPromptEvaluatorTools(1: GetDefaultPromptEvaluatorToolsRequest req) (api.post="/api/evaluation/v1/evaluators/default_prompt_evaluator_tools") // 获取prompt evaluator tools配置

    // 评估器执行
    RunEvaluatorResponse RunEvaluator(1: RunEvaluatorRequest req) (api.post="/api/evaluation/v1/evaluators_versions/:evaluator_version_id/run")// evaluator 运行
    DebugEvaluatorResponse DebugEvaluator(1: DebugEvaluatorRequest req) (api.post="/api/evaluation/v1/evaluators/debug")// evaluator 调试

    // 评估器执行结果
    UpdateEvaluatorRecordResponse UpdateEvaluatorRecord(1: UpdateEvaluatorRecordRequest req) (api.patch="/api/evaluation/v1/evaluator_records/:evaluator_record_id") // 修正evaluator运行分数
    GetEvaluatorRecordResponse GetEvaluatorRecord(1: GetEvaluatorRecordRequest req)
    BatchGetEvaluatorRecordsResponse BatchGetEvaluatorRecords(1: BatchGetEvaluatorRecordsRequest req)

} (api.js_conv="true" )