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

// 实验类型
typedef string ExperimentType(ts.enum="true")
const ExperimentType ExperimentType_Offline = "offline"
const ExperimentType ExperimentType_Online = "online"

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
    1: optional string evaluator_version_id
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
    1: optional string evaluator_version_id
    2: optional string evaluator_name
    3: optional double average_score
    4: optional double max_score
    5: optional double min_score
    6: optional i32 success_count
    7: optional i32 failed_count
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
    4: optional string creator_by

    // 运行信息
    10: optional ExperimentStatus status // 实验状态
    12: optional i64 start_time  (api.js_conv='true', go.tag='json:"start_time"') // ISO 8601格式
    13: optional i64 end_time    (api.js_conv='true', go.tag='json:"start_time"') // ISO 8601格式
    14: optional i32 item_concur_num // 评测集并发数
    15: optional i32 evaluators_concur_num // 评估器并发数
    16: optional common.RuntimeParam target_runtime_param   // 运行时参数

    // 三元组信息
    30: optional string eval_set_version_id
    31: optional string target_version_id
    32: optional list<string> evaluator_version_ids
    33: optional TargetFieldMapping target_field_mapping
    34: optional list<EvaluatorFieldMapping> evaluator_field_mapping

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

// 评估器输出结果
struct EvaluatorOutput {
    1: optional map<string, evaluator.EvaluatorRecord> evaluator_records  // key为evaluator_version_id
}

// 结果payload
struct ResultPayload {
    1: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    2: optional eval_set.Turn eval_set_turn
    3: optional eval_target.EvalTargetRecord target_output
    4: optional EvaluatorOutput evaluator_output
}

// 轮次结果
struct TurnResult {
    1: optional string turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    2: optional ResultPayload payload
}

// 数据项结果
struct ItemResult {
    1: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"')
    2: optional list<TurnResult> turn_results
}