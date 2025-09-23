namespace go coze.loop.evaluation.openapi

include "../../../base.thrift"
include "domain_openapi/common.thrift"
include "domain_openapi/eval_set.thrift"
include "domain_openapi/evaluator.thrift"
include "domain_openapi/experiment.thrift"

// ===============================
// 评测集相关接口
// ===============================

// 创建评测集
struct CreateEvaluationSetRequest {
    1: optional string name
    2: optional string description
    3: optional eval_set.EvaluationSetSchema evaluation_set_schema
    4: optional string biz_category

    255: optional base.Base Base
}

struct CreateEvaluationSetResponse {
    1: optional eval_set.EvaluationSet evaluation_set

    255: base.BaseResp BaseResp
}

// 获取评测集
struct GetEvaluationSetRequest {
    1: required string evaluation_set_id (api.path = "evaluation_set_id")

    255: optional base.Base Base
}

struct GetEvaluationSetResponse {
    1: optional eval_set.EvaluationSet evaluation_set

    255: base.BaseResp BaseResp
}

// 更新评测集
struct UpdateEvaluationSetRequest {
    1: required string evaluation_set_id (api.path = "evaluation_set_id")
    2: optional string name
    3: optional string description

    255: optional base.Base Base
}

struct UpdateEvaluationSetResponse {
    1: optional eval_set.EvaluationSet evaluation_set

    255: base.BaseResp BaseResp
}

// 删除评测集
struct DeleteEvaluationSetRequest {
    1: required string evaluation_set_id (api.path = "evaluation_set_id")

    255: optional base.Base Base
}

struct DeleteEvaluationSetResponse {
    255: base.BaseResp BaseResp
}

// 列出评测集
struct ListEvaluationSetsRequest {
    1: optional string name
    2: optional list<string> creators
    3: optional list<string> evaluation_set_ids
    4: optional i32 page_num
    5: optional i32 page_size

    255: optional base.Base Base
}

struct ListEvaluationSetsResponse {
    1: optional list<eval_set.EvaluationSet> evaluation_sets
    2: optional common.PageInfo page_info

    255: base.BaseResp BaseResp
}

// ===============================
// 评估器相关接口
// ===============================

// 创建评估器
struct CreateEvaluatorRequest {
    1: optional string name
    2: optional string description
    3: optional evaluator.EvaluatorType evaluator_type
    4: optional evaluator.EvaluatorContent evaluator_content

    255: optional base.Base Base
}

struct CreateEvaluatorResponse {
    1: optional evaluator.Evaluator evaluator

    255: base.BaseResp BaseResp
}

// 获取评估器
struct GetEvaluatorRequest {
    1: required string evaluator_id (api.path = "evaluator_id")

    255: optional base.Base Base
}

struct GetEvaluatorResponse {
    1: optional evaluator.Evaluator evaluator

    255: base.BaseResp BaseResp
}

// 更新评估器
struct UpdateEvaluatorRequest {
    1: required string evaluator_id (api.path = "evaluator_id")
    2: optional string name
    3: optional string description
    4: optional evaluator.EvaluatorContent evaluator_content

    255: optional base.Base Base
}

struct UpdateEvaluatorResponse {
    1: optional evaluator.Evaluator evaluator

    255: base.BaseResp BaseResp
}

// 删除评估器
struct DeleteEvaluatorRequest {
    1: required string evaluator_id (api.path = "evaluator_id")

    255: optional base.Base Base
}

struct DeleteEvaluatorResponse {
    255: base.BaseResp BaseResp
}

// 列出评估器
struct ListEvaluatorsRequest {
    1: optional string name
    2: optional evaluator.EvaluatorType evaluator_type
    3: optional list<string> creators
    4: optional i32 page_num
    5: optional i32 page_size

    255: optional base.Base Base
}

struct ListEvaluatorsResponse {
    1: optional list<evaluator.Evaluator> evaluators
    2: optional common.PageInfo page_info

    255: base.BaseResp BaseResp
}

// ===============================
// 评测实验相关接口
// ===============================

// 创建评测实验
struct CreateExperimentRequest {
    1: optional string name
    2: optional string description
    3: optional string eval_set_version_id
    4: optional string target_version_id
    5: optional list<string> evaluator_version_ids
    6: optional experiment.TargetFieldMapping target_field_mapping
    7: optional list<experiment.EvaluatorFieldMapping> evaluator_field_mapping
    8: optional i32 item_concur_num
    9: optional i32 evaluators_concur_num
    10: optional experiment.ExperimentType experiment_type

    255: optional base.Base Base
}

struct CreateExperimentResponse {
    1: optional experiment.Experiment experiment

    255: base.BaseResp BaseResp
}

// 获取评测实验
struct GetExperimentRequest {
    1: required string experiment_id (api.path = "experiment_id")

    255: optional base.Base Base
}

struct GetExperimentResponse {
    1: optional experiment.Experiment experiment

    255: base.BaseResp BaseResp
}

// 列出评测实验
struct ListExperimentsRequest {
    1: optional string name
    2: optional experiment.ExperimentStatus status
    3: optional list<string> creators
    4: optional i32 page_num
    5: optional i32 page_size

    255: optional base.Base Base
}

struct ListExperimentsResponse {
    1: optional list<experiment.Experiment> experiments
    2: optional common.PageInfo page_info

    255: base.BaseResp BaseResp
}

// 启动评测实验
struct StartExperimentRequest {
    1: required string experiment_id (api.path = "experiment_id")

    255: optional base.Base Base
}

struct StartExperimentResponse {
    1: optional experiment.Experiment experiment

    255: base.BaseResp BaseResp
}

// 停止评测实验
struct StopExperimentRequest {
    1: required string experiment_id (api.path = "experiment_id")

    255: optional base.Base Base
}

struct StopExperimentResponse {
    1: optional experiment.Experiment experiment

    255: base.BaseResp BaseResp
}

// 获取实验结果
struct GetExperimentResultsRequest {
    1: required string experiment_id (api.path = "experiment_id")
    2: optional i32 page_num
    3: optional i32 page_size

    255: optional base.Base Base
}

struct GetExperimentResultsResponse {
    1: optional list<experiment.ItemResult> item_results
    2: optional common.PageInfo page_info

    255: base.BaseResp BaseResp
}

// ===============================
// 服务定义
// ===============================

service EvaluationOpenAPIService {
    // 评测集接口
    CreateEvaluationSetResponse CreateEvaluationSet(1: CreateEvaluationSetRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets")
    GetEvaluationSetResponse GetEvaluationSet(1: GetEvaluationSetRequest req) (api.get = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id")
    UpdateEvaluationSetResponse UpdateEvaluationSet(1: UpdateEvaluationSetRequest req) (api.put = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id")
    DeleteEvaluationSetResponse DeleteEvaluationSet(1: DeleteEvaluationSetRequest req) (api.delete = "/open-apis/evaluation/v1/evaluation_sets/:evaluation_set_id")
    ListEvaluationSetsResponse ListEvaluationSets(1: ListEvaluationSetsRequest req) (api.post = "/open-apis/evaluation/v1/evaluation_sets/list")

    // 评估器接口
    CreateEvaluatorResponse CreateEvaluator(1: CreateEvaluatorRequest req) (api.post = "/open-apis/evaluation/v1/evaluators")
    GetEvaluatorResponse GetEvaluator(1: GetEvaluatorRequest req) (api.get = "/open-apis/evaluation/v1/evaluators/:evaluator_id")
    UpdateEvaluatorResponse UpdateEvaluator(1: UpdateEvaluatorRequest req) (api.put = "/open-apis/evaluation/v1/evaluators/:evaluator_id")
    DeleteEvaluatorResponse DeleteEvaluator(1: DeleteEvaluatorRequest req) (api.delete = "/open-apis/evaluation/v1/evaluators/:evaluator_id")
    ListEvaluatorsResponse ListEvaluators(1: ListEvaluatorsRequest req) (api.post = "/open-apis/evaluation/v1/evaluators/list")

    // 评测实验接口
    CreateExperimentResponse CreateExperiment(1: CreateExperimentRequest req) (api.post = "/open-apis/evaluation/v1/experiments")
    GetExperimentResponse GetExperiment(1: GetExperimentRequest req) (api.get = "/open-apis/evaluation/v1/experiments/:experiment_id")
    ListExperimentsResponse ListExperiments(1: ListExperimentsRequest req) (api.post = "/open-apis/evaluation/v1/experiments/list")
    StartExperimentResponse StartExperiment(1: StartExperimentRequest req) (api.post = "/open-apis/evaluation/v1/experiments/:experiment_id/start")
    StopExperimentResponse StopExperiment(1: StopExperimentRequest req) (api.post = "/open-apis/evaluation/v1/experiments/:experiment_id/stop")
    GetExperimentResultsResponse GetExperimentResults(1: GetExperimentResultsRequest req) (api.get = "/open-apis/evaluation/v1/experiments/:experiment_id/results")
}