namespace go coze.loop.evaluation.domain_openapi.experiment

include "common.thrift"
include "eval_set.thrift"
include "evaluator.thrift"
include "eval_target.thrift"

// 实验状态
typedef string ExperimentStatus(ts.enum="true")
const ExperimentStatus ExperimentStatus_Pending = "pending"
const ExperimentStatus ExperimentStatus_Processing = "processing"
const ExperimentStatus ExperimentStatus_Success = "success"
const ExperimentStatus ExperimentStatus_Failed = "failed"
const ExperimentStatus ExperimentStatus_Terminated = "terminated"
const ExperimentStatus ExperimentStatus_SystemTerminated = "system_terminated"
const ExperimentStatus ExperimentStatus_Draining = "draining"

// 实验类型
typedef string ExperimentType(ts.enum="true")
const ExperimentType ExperimentType_Offline = "offline"
const ExperimentType ExperimentType_Online = "online"

// 聚合器类型
typedef string AggregatorType(ts.enum="true")
const AggregatorType AggregatorType_Average = "average"
const AggregatorType AggregatorType_Sum = "sum"
const AggregatorType AggregatorType_Max = "max"
const AggregatorType AggregatorType_Min = "min"
const AggregatorType AggregatorType_Distribution = "distribution"

// 数据类型
typedef string DataType(ts.enum="true")
const DataType DataType_Double = "double"
const DataType DataType_ScoreDistribution = "score_distribution"

typedef string ItemRunState(ts.enum="true")
const ItemRunState ItemRunState_Queueing = "queueing"
const ItemRunState ItemRunState_Processing = "processing"
const ItemRunState ItemRunState_Success = "success"
const ItemRunState ItemRunState_Fail = "fail"
const ItemRunState ItemRunState_Terminal = "terminal"


typedef string TurnRunState(ts.enum="true")
const TurnRunState TurnRunState_Queueing = "queueing"
const TurnRunState TurnRunState_Processing = "processing"
const TurnRunState TurnRunState_Success = "success"
const TurnRunState TurnRunState_Fail = "fail"
const TurnRunState TurnRunState_Terminal = "terminal"


// 字段映射
struct FieldMapping {
    1: optional string field_name
    2: optional string from_field_name
}

// 目标字段映射
struct TargetFieldMapping {
    1: optional list<FieldMapping> from_eval_set
}

// 评估器字段映射
struct EvaluatorFieldMapping {
    1: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    2: optional list<FieldMapping> from_eval_set
    3: optional list<FieldMapping> from_target
}

// Token使用量
struct TokenUsage {
    1: optional string input_tokens
    2: optional string output_tokens
}

// 评估器聚合结果
struct EvaluatorAggregateResult {
    1: optional i64 evaluator_id (api.js_conv = 'true', go.tag = 'json:"evaluator_id"')
    2: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    3: optional string name
    4: optional string version

    20: optional list<AggregatorResult> aggregator_results
}

// 一种聚合器类型的聚合结果
struct  AggregatorResult {
    1: optional AggregatorType aggregator_type
    2: optional AggregateData data
}

struct AggregateData {
    1: optional DataType data_type
    2: optional double value
    3: optional ScoreDistribution score_distribution
}

struct ScoreDistribution {
    1: optional list<ScoreDistributionItem> score_distribution_items
}

struct ScoreDistributionItem {
    1: optional string score
    2: optional i64 count (api.js_conv='true', go.tag='json:"count"')
    3: optional double percentage
}

// 实验统计
struct ExperimentStatistics {
    1: optional i32 pending_turn_count
    2: optional i32 success_turn_count
    3: optional i32 failed_turn_count
    4: optional i32 terminated_turn_count
    5: optional i32 processing_turn_count
}

// 评测实验
struct Experiment {
    // 基本信息
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')
    2: optional string name
    3: optional string description

    // 运行信息
    10: optional ExperimentStatus status // 实验状态
    11: optional i64 start_time  (api.js_conv='true', go.tag='json:"start_time"') // ISO 8601格式
    12: optional i64 end_time    (api.js_conv='true', go.tag='json:"end_time"') // ISO 8601格式
    13: optional i32 item_concur_num // 评测集并发数
    14: optional common.RuntimeParam target_runtime_param   // 运行时参数

    // 三元组信息
    31: optional TargetFieldMapping target_field_mapping
    32: optional list<EvaluatorFieldMapping> evaluator_field_mapping

    // 统计信息
    50: optional ExperimentStatistics expt_stats

    100: optional common.BaseInfo base_info
}

// 列定义 - 评测集字段
struct ColumnEvalSetField {
    1: optional string key
    2: optional string name
    3: optional string description
    4: optional common.ContentType content_type
    6: optional string text_schema
}

// 列定义 - 评估器
struct ColumnEvaluator {
    1: optional i64 evaluator_version_id (api.js_conv='true', go.tag='json:"evaluator_version_id"')
    2: optional i64 evaluator_id (api.js_conv='true', go.tag='json:"evaluator_id"')
    3: optional evaluator.EvaluatorType evaluator_type
    4: optional string name
    5: optional string version
    6: optional string description
}

// 目标输出结果
struct TargetOutput {
    1: optional string target_record_id
    2: optional evaluator.EvaluatorRunStatus status
    3: optional map<string, common.Content> output_fields
    4: optional string time_consuming_ms
    5: optional evaluator.EvaluatorRunError error
}

// 结果payload
struct ResultPayload {
    1: optional eval_set.Turn eval_set_turn // 评测集行数据信息
    2: optional eval_target.EvalTargetRecord target_record  // 评测对象执行结果
    3: optional list<evaluator.EvaluatorRecord> evaluator_records   // 评估器执行结果列表

    20: optional TurnSystemInfo system_info
}

struct TurnSystemInfo {
    1: optional TurnRunState turn_run_state
}

// 轮次结果
struct TurnResult {
    1: optional string turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    2: optional ResultPayload payload
}

// 数据项结果
struct ItemResult {
    1: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"')   // 数据项(行)ID
    2: optional list<TurnResult> turn_results   // 轮次结果，单轮仅有一个元素

    20: optional ItemSystemInfo system_info
}

struct ItemSystemInfo {
    1: optional ItemRunState run_state
}