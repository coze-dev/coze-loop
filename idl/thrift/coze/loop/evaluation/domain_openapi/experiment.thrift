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
    1: optional i64 evaluator_id (api.js_conv="true", go.tag='json:"evaluator_id"')
    2: optional string version
    3: optional list<FieldMapping> from_eval_set
    4: optional list<FieldMapping> from_target
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

struct EvalTargetAggregateResult {
    1: optional i64 target_id (api.js_conv = 'true')
    2: optional i64 target_version_id (api.js_conv = 'true')

    5: optional list<AggregatorResult> latency
    6: optional list<AggregatorResult> input_tokens
    7: optional list<AggregatorResult> output_tokens
    8: optional list<AggregatorResult> total_tokens
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
    11: optional i64 started_at  (api.js_conv='true', go.tag='json:"started_at"') // ISO 8601格式
    12: optional i64 ended_at    (api.js_conv='true', go.tag='json:"ended_at"') // ISO 8601格式
    13: optional i32 item_concur_num // 评测集并发数
    14: optional common.RuntimeParam target_runtime_param   // 运行时参数

    // 三元组信息
    31: optional TargetFieldMapping target_field_mapping
    32: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    33: optional eval_set.EvaluationSet eval_set
    34: optional eval_target.EvalTarget eval_target

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

const string ColumnEvalTargetName_ActualOutput = "actual_output"
const string ColumnEvalTargetName_Trajectory = "trajectory"
const string ColumnEvalTargetName_EvalTargetTotalLatency = "eval_target_total_latency"
const string ColumnEvalTargetName_EvaluatorInputTokens = "eval_target_input_tokens"
const string ColumnEvalTargetName_EvaluatorOutputTokens = "eval_target_output_tokens"
const string ColumnEvalTargetName_EvaluatorTotalTokens = "eval_target_total_tokens"

struct ColumnEvalTarget {
    1: optional string name
    2: optional string description
    3: optional string label
    4: optional common.ContentType content_type
    5: optional string text_schema
    6: optional eval_set.SchemaKey schema_key
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

// ===============================
// 实验模板相关结构定义
// ===============================

// 实验模板基础信息
struct ExptTemplateMeta {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    3: optional string name
    4: optional string description
    5: optional ExperimentType expt_type   // 模板对应的实验类型，当前主要为 Offline
}

// 实验三元组配置
struct ExptTuple {
    1: optional i64 eval_set_id (api.js_conv='true', go.tag='json:"eval_set_id"')
    2: optional i64 eval_set_version_id (api.js_conv='true', go.tag='json:"eval_set_version_id"')
    3: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    4: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    5: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_items (go.tag = 'json:"evaluator_id_version_items"')

    // 兼容内部结构
    7: optional eval_set.EvaluationSet eval_set
    8: optional eval_target.EvalTarget eval_target
    9: optional list<evaluator.Evaluator> evaluators
}

// 实验模板字段映射配置
struct ExptFieldMapping {
    1: optional TargetFieldMapping target_field_mapping
    2: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    3: optional common.RuntimeParam target_runtime_param
    4: optional i32 item_concur_num
}

// 实验评估器得分加权配置（evaluator_id -> weight）
struct ExptScoreWeight {
    1: optional bool enable_weighted_score (go.tag = 'json:"enable_weighted_score"')
    2: optional map<i64, double> evaluator_score_weights (api.js_conv = "true", go.tag = 'json:"evaluator_score_weights"')
}

// 实验模板
struct ExptTemplate {
    1: optional ExptTemplateMeta meta
    2: optional ExptTuple triple_config
    3: optional ExptFieldMapping field_mapping_config
    4: optional ExptScoreWeight score_weight_config (go.tag = 'json:"score_weight_config"')

    100: optional common.BaseInfo base_info
}

// ===============================
// 筛选能力结构（与 domain/expt.thrift 结构一致）
// ===============================

// 筛选逻辑操作符（对应 domain/expt FilterLogicOp）
typedef string FilterLogicOp(ts.enum="true")
const FilterLogicOp FilterLogicOp_Unknown = "unknown"
const FilterLogicOp FilterLogicOp_And = "and"
const FilterLogicOp FilterLogicOp_Or = "or"

// 筛选操作符类型（对应 domain/expt FilterOperatorType）
typedef string FilterOperatorType(ts.enum="true")
const FilterOperatorType FilterOperatorType_Unknown = "unknown"
const FilterOperatorType FilterOperatorType_Equal = "equal"
const FilterOperatorType FilterOperatorType_NotEqual = "not_equal"
const FilterOperatorType FilterOperatorType_Greater = "greater"
const FilterOperatorType FilterOperatorType_GreaterOrEqual = "greater_or_equal"
const FilterOperatorType FilterOperatorType_Less = "less"
const FilterOperatorType FilterOperatorType_LessOrEqual = "less_or_equal"
const FilterOperatorType FilterOperatorType_In = "in"
const FilterOperatorType FilterOperatorType_NotIn = "not_in"
const FilterOperatorType FilterOperatorType_Like = "like"
const FilterOperatorType FilterOperatorType_NotLike = "not_like"
const FilterOperatorType FilterOperatorType_IsNull = "is_null"
const FilterOperatorType FilterOperatorType_IsNotNull = "is_not_null"

// 筛选字段类型（对应 domain/expt FieldType）
typedef string FilterFieldType(ts.enum="true")
const FilterFieldType FilterFieldType_Unknown = "unknown"
const FilterFieldType FilterFieldType_CreatorBy = "creator_by"
const FilterFieldType FilterFieldType_UpdatedBy = "updated_by"
const FilterFieldType FilterFieldType_EvalSetID = "eval_set_id"
const FilterFieldType FilterFieldType_TargetID = "target_id"
const FilterFieldType FilterFieldType_EvaluatorID = "evaluator_id"
const FilterFieldType FilterFieldType_TargetType = "target_type"
const FilterFieldType FilterFieldType_ExptType = "expt_type"
const FilterFieldType FilterFieldType_Name = "name"  // 模板名称模糊搜索

// 筛选字段（对应 domain/expt FilterField）
struct FilterField {
    1: optional FilterFieldType field_type
    2: optional string field_key  // 二级key
}

// 筛选条件（对应 domain/expt FilterCondition）
struct FilterCondition {
    1: optional FilterField field
    2: optional FilterOperatorType operator
    3: optional string value
}

// 关键词搜索（对应 domain/expt KeywordSearch）
struct KeywordSearch {
    1: optional string keyword
    2: optional list<FilterField> filter_fields
}

// 通用筛选逻辑（对应 domain/expt Filters）
struct Filters {
    1: optional list<FilterCondition> filter_conditions
    2: optional FilterLogicOp logic_op
}

// 实验模板筛选器（对应 domain/expt ExperimentTemplateFilter）
struct ExperimentTemplateFilter {
    1: optional Filters filters
    2: optional KeywordSearch keyword_search
}
