namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "domain_openapi/common.thrift"
include "domain_openapi/eval_set.thrift"
include "domain_openapi/evaluator.thrift"
include "domain_openapi/experiment.thrift"

// ===============================
// 评测集相关接口 (9个接口)
// ===============================

// 1.1 创建评测集
struct CreateEvaluationSetOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string name (vt.min_size = "1", vt.max_size = "255")
    3: optional string description (vt.max_size = "2048")
    4: optional eval_set.EvaluationSetSchema evaluation_set_schema
    5: optional string biz_category (vt.max_size = "128")

    255: optional base.Base Base
}

struct CreateEvaluationSetOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluationSetOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetOpenAPIData {
    1: optional string evaluation_set_id (api.js_conv="true")
}

// 1.2 获取评测集详情
struct GetEvaluationSetOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path = "evaluation_set_id")
    3: optional bool include_deleted

    255: optional base.Base Base
}

struct GetEvaluationSetOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluationSetOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct GetEvaluationSetOpenAPIData {
    1: optional eval_set.EvaluationSet evaluation_set
}

// 1.3 查询评测集列表
struct ListEvaluationSetsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string name
    3: optional list<string> creators
    4: optional string page_token
    5: optional i32 page_size (vt.gt = "0", vt.le = "200")
    
    255: optional base.Base Base
}

struct ListEvaluationSetsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct ListEvaluationSetsOpenAPIData {
    1: optional list<eval_set.EvaluationSet> items
    2: optional bool has_more
    3: optional string next_page_token
    4: optional i64 total
}

// 1.4 创建评测集版本
struct CreateEvaluationSetVersionOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path = "evaluation_set_id")
    3: optional string version (vt.min_size = "1", vt.max_size="50")
    4: optional string description (vt.max_size = "400")
    
    255: optional base.Base Base
}

struct CreateEvaluationSetVersionOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluationSetVersionOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct CreateEvaluationSetVersionOpenAPIData {
    1: optional string version_id (api.js_conv="true")
}

// 1.5 批量添加评测集数据
struct BatchCreateEvaluationSetItemsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path='evaluation_set_id')
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size='1',vt.max_size='100')
    4: optional bool skip_invalid_items
    5: optional bool allow_partial_add
    
    255: optional base.Base Base
}

struct BatchCreateEvaluationSetItemsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchCreateEvaluationSetItemsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct BatchCreateEvaluationSetItemsOpenAPIData {
    1: optional map<i64, string> added_items
    2: optional list<eval_set.ItemErrorGroup> errors
}

// 1.6 批量更新评测集数据
struct BatchUpdateEvaluationSetItemsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path='evaluation_set_id')
    3: optional list<eval_set.EvaluationSetItem> items (vt.min_size='1',vt.max_size='100')
    4: optional bool skip_invalid_items
    
    255: optional base.Base Base
}

struct BatchUpdateEvaluationSetItemsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchUpdateEvaluationSetItemsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct BatchUpdateEvaluationSetItemsOpenAPIData {
    1: optional map<i64, string> updated_items
    2: optional list<eval_set.ItemErrorGroup> errors
}

// 1.7 批量删除评测集数据
struct BatchDeleteEvaluationSetItemsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path = "evaluation_set_id")
    3: optional list<string> item_ids
    
    255: optional base.Base Base
}

struct BatchDeleteEvaluationSetItemsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional BatchDeleteEvaluationSetItemsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct BatchDeleteEvaluationSetItemsOpenAPIData {
    1: optional i32 deleted_count
}

// 1.8 清空评测集草稿数据
struct ClearEvaluationSetDraftItemsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path = "evaluation_set_id")
    
    255: optional base.Base Base
}

struct ClearEvaluationSetDraftItemsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ClearEvaluationSetDraftItemsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct ClearEvaluationSetDraftItemsOpenAPIData {
    1: optional i32 cleared_count
}

// 1.9 查询评测集特定版本数据
struct ListEvaluationSetVersionItemsOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluation_set_id (api.path = "evaluation_set_id")
    3: required string version_id (api.path = "version_id")
    4: optional string page_token
    5: optional i32 page_size (vt.gt = "0", vt.le = "200")
    
    255: optional base.Base Base
}

struct ListEvaluationSetVersionItemsOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional ListEvaluationSetVersionItemsOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct ListEvaluationSetVersionItemsOpenAPIData {
    1: optional list<eval_set.EvaluationSetItem> items
    2: optional bool has_more
    3: optional string next_page_token
    4: optional i64 total
}

// ===============================
// 评估器相关接口 (5个接口)
// ===============================

// 2.1 创建评估器
struct CreateEvaluatorOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required evaluator.Evaluator evaluator
    
    255: optional base.Base Base
}

struct CreateEvaluatorOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateEvaluatorOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct CreateEvaluatorOpenAPIData {
    1: optional string evaluator_id (api.js_conv='true')
}

// 2.2 提交评估器版本
struct SubmitEvaluatorVersionOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_id (api.path='evaluator_id')
    3: required string version
    4: optional string description
    
    255: optional base.Base Base
}

struct SubmitEvaluatorVersionOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional SubmitEvaluatorVersionOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct SubmitEvaluatorVersionOpenAPIData {
    1: optional evaluator.Evaluator evaluator
}

// 2.3 获取评估器版本详情
struct GetEvaluatorVersionOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_version_id (api.path='evaluator_version_id')
    3: optional bool include_deleted
    
    255: optional base.Base Base
}

struct GetEvaluatorVersionOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluatorVersionOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct GetEvaluatorVersionOpenAPIData {
    1: optional evaluator.Evaluator evaluator
}

// 2.4 执行评估器
struct RunEvaluatorOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_version_id (api.path='evaluator_version_id')
    3: required evaluator.EvaluatorInputData input_data
    4: optional map<string, string> ext
    
    255: optional base.Base Base
}

struct RunEvaluatorOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional RunEvaluatorOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct RunEvaluatorOpenAPIData {
    1: required evaluator.EvaluatorRecord record
}

// 2.5 获取评估器执行结果
struct GetEvaluatorRecordOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string evaluator_record_id (api.path='evaluator_record_id')
    3: optional bool include_deleted
    
    255: optional base.Base Base
}

struct GetEvaluatorRecordOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetEvaluatorRecordOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct GetEvaluatorRecordOpenAPIData {
    1: required evaluator.EvaluatorRecord record
}

// ===============================
// 评测实验相关接口 (2个接口)
// ===============================

// 3.1 创建评测实验
struct CreateExperimentOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: optional string eval_set_version_id
    3: optional string target_version_id
    4: optional list<string> evaluator_version_ids
    5: optional string name
    6: optional string description
    7: optional experiment.TargetFieldMapping target_field_mapping
    8: optional list<experiment.EvaluatorFieldMapping> evaluator_field_mapping
    9: optional i32 item_concur_num
    10: optional i32 evaluators_concur_num
    
    255: optional base.Base Base
}

struct CreateExperimentOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional CreateExperimentOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct CreateExperimentOpenAPIData {
    1: optional experiment.Experiment experiment
}

// 3.2 获取评测实验结果
struct GetExperimentResultOpenAPIRequest {
    1: required string workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    2: required string experiment_id (api.path = "experiment_id")
    3: optional string page_token
    4: optional i32 page_size (vt.gt = "0", vt.le = "200")
    
    255: optional base.Base Base
}

struct GetExperimentResultOpenAPIResponse {
    1: optional i32 code
    2: optional string msg
    3: optional GetExperimentResultOpenAPIData data
    
    255: base.BaseResp BaseResp
}

struct GetExperimentResultOpenAPIData {
    1: required list<experiment.ColumnEvalSetField> column_eval_set_fields
    2: optional list<experiment.ColumnEvaluator> column_evaluators
    3: optional list<experiment.ItemResult> item_results
    4: optional bool has_more
    5: optional string next_page_token
    6: optional i64 total
}

// ===============================
// 服务定义 (16个接口总计)
// ===============================

service EvaluationOpenAPIService {
    // 评测集接口 (9个)
    // 1.1 创建评测集
    CreateEvaluationSetOpenAPIResponse CreateEvaluationSet(1: CreateEvaluationSetOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets")
    // 1.2 获取评测集详情
    GetEvaluationSetOpenAPIResponse GetEvaluationSet(1: GetEvaluationSetOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id")
    // 1.3 查询评测集列表
    ListEvaluationSetsOpenAPIResponse ListEvaluationSets(1: ListEvaluationSetsOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/evaluation_sets")
    // 1.4 创建评测集版本
    CreateEvaluationSetVersionOpenAPIResponse CreateEvaluationSetVersion(1: CreateEvaluationSetVersionOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/versions")
    // 1.5 批量添加评测集数据
    BatchCreateEvaluationSetItemsOpenAPIResponse BatchCreateEvaluationSetItems(1: BatchCreateEvaluationSetItemsOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items")
    // 1.6 批量更新评测集数据
    BatchUpdateEvaluationSetItemsOpenAPIResponse BatchUpdateEvaluationSetItems(1: BatchUpdateEvaluationSetItemsOpenAPIRequest req) (api.put = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items")
    // 1.7 批量删除评测集数据
    BatchDeleteEvaluationSetItemsOpenAPIResponse BatchDeleteEvaluationSetItems(1: BatchDeleteEvaluationSetItemsOpenAPIRequest req) (api.delete = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items")
    // 1.8 清空评测集草稿数据
    ClearEvaluationSetDraftItemsOpenAPIResponse ClearEvaluationSetDraftItems(1: ClearEvaluationSetDraftItemsOpenAPIRequest req) (api.delete = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/items/draft")
    // 1.9 查询评测集特定版本数据
    ListEvaluationSetVersionItemsOpenAPIResponse ListEvaluationSetVersionItems(1: ListEvaluationSetVersionItemsOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id/versions/:version_id/items")

    // 评估器接口 (5个)
    // 2.1 创建评估器
    CreateEvaluatorOpenAPIResponse CreateEvaluator(1: CreateEvaluatorOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluators")
    // 2.2 提交评估器版本
    SubmitEvaluatorVersionOpenAPIResponse SubmitEvaluatorVersion(1: SubmitEvaluatorVersionOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluators/:evaluator_id/versions")
    // 2.3 获取评估器版本详情
    GetEvaluatorVersionOpenAPIResponse GetEvaluatorVersion(1: GetEvaluatorVersionOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/evaluators/versions/:evaluator_version_id")
    // 2.4 执行评估器
    RunEvaluatorOpenAPIResponse RunEvaluator(1: RunEvaluatorOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/evaluators/versions/:evaluator_version_id/run")
    // 2.5 获取评估器执行结果
    GetEvaluatorRecordOpenAPIResponse GetEvaluatorRecord(1: GetEvaluatorRecordOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/evaluator_records/:evaluator_record_id")

    // 评测实验接口 (2个)
    // 3.1 创建评测实验
    CreateExperimentOpenAPIResponse CreateExperiment(1: CreateExperimentOpenAPIRequest req) (api.post = "/open-apis/evaluation/v1/experiments")
    // 3.2 获取评测实验结果
    GetExperimentResultOpenAPIResponse GetExperimentResult(1: GetExperimentResultOpenAPIRequest req) (api.get = "/open-apis/evaluation/v1/experiments/:experiment_id/results")
}