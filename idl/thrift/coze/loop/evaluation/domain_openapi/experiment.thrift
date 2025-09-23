namespace go coze.loop.evaluation.domain_openapi.experiment

include "common.thrift"
include "eval_set.thrift"
include "evaluator.thrift"

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
    3: optional string const_value
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
    1: optional list<EvaluatorAggregateResult> evaluator_aggregate_results
    2: optional TokenUsage token_usage
    3: optional double credit_cost
    4: optional i32 pending_turn_count
    5: optional i32 success_turn_count
    6: optional i32 failed_turn_count
    7: optional i32 terminated_turn_count
    8: optional i32 processing_turn_count
}

// 评测实验
struct Experiment {
    1: optional string experiment_id
    2: optional string name
    3: optional string description
    4: optional ExperimentStatus status
    5: optional string status_message
    6: optional string start_time  // ISO 8601格式
    7: optional string end_time    // ISO 8601格式
    8: optional string eval_set_version_id
    9: optional string target_version_id
    10: optional list<string> evaluator_version_ids
    11: optional TargetFieldMapping target_field_mapping
    12: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    13: optional i32 item_concur_num
    14: optional i32 evaluators_concur_num
    15: optional ExperimentType experiment_type
    16: optional ExperimentStatistics experiment_statistics
    17: optional common.BaseInfo base_info
}

// 列定义 - 评测集字段
struct ColumnEvalSetField {
    1: optional string key
    2: optional string name
    3: optional string description
    4: optional common.ContentType content_type
}

// 列定义 - 评估器
struct ColumnEvaluator {
    1: optional string evaluator_version_id
    2: optional string evaluator_id
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

// 实验结果载荷
struct ExperimentResultPayload {
    1: optional string turn_id
    2: optional eval_set.Turn eval_set_turn
    3: optional TargetOutput target_output
    4: optional EvaluatorOutput evaluator_output
}

// 轮次结果
struct TurnResult {
    1: optional string turn_id
    2: optional list<ExperimentResult> experiment_results
}

// 实验结果
struct ExperimentResult {
    1: optional string experiment_id
    2: optional ExperimentResultPayload payload
}

// 数据项结果
struct ItemResult {
    1: optional string item_id
    2: optional list<TurnResult> turn_results
}