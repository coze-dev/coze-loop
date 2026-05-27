namespace go coze.loop.evaluation.domain.expt

include "common.thrift"
include "eval_target.thrift"
include "evaluator.thrift"
include "eval_set.thrift"
include "../../data/domain/tag.thrift"
include "../../data/domain/dataset.thrift"
include "../../observability/domain/filter.thrift"
include "../../observability/domain/task.thrift"

enum ExptStatus {
    Unknown = 0

    Pending = 2    // Awaiting execution
    Processing = 3 // In progress

    Success = 11   // Execution succeeded
    Failed = 12    // Execution failed
    Terminated = 13   // User terminated
    SystemTerminated = 14 // System terminated
    Terminating = 15 // Terminating

    Draining = 21 // online expt draining
}

enum ExptType {
    Offline = 1
    Online = 2
}

// 离线实验分析状态（与表字段 offline_expt_analysis_status 一致）
enum OfflineExptAnalysisStatus {
    NotStarted = 0  // 未开始
    Processing = 1  // 进行中
    Success = 2    // 成功
    Failed = 3     // 失败
    Superseded = 4 // 已被新版本/新分析取代
}

enum SourceType {
    Evaluation = 1
    AutoTask = 2
    Workflow = 3
    IntelligentGen =4    // 智能生成
}

typedef string Visibility(ts.enum="true")
const Visibility Visibility_Hidden = "hidden"


typedef string ExptTriggerType (ts.enum="true")
const ExptTriggerType Manual = "manual"
const ExptTriggerType OpenAPI = "openapi"
const ExptTriggerType Schedule = "schedule"

struct Experiment {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')
    2: optional string name
    3: optional string desc
    4: optional string creator_by
    5: optional ExptStatus status
    6: optional string status_message
    7: optional i64 start_time (api.js_conv='true', go.tag='json:"start_time"')
    8: optional i64 end_time (api.js_conv='true', go.tag='json:"end_time"')
    9: optional i32 item_concur_num
    10: optional Visibility visibility  // 实验可见性，默认为空，可见

    21: optional i64 eval_set_version_id (api.js_conv='true', go.tag='json:"eval_set_version_id"')
    22: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    23: optional list<i64> evaluator_version_ids (api.js_conv='true', go.tag='json:"evaluator_version_ids"')
    24: optional eval_set.EvaluationSet eval_set
    25: optional eval_target.EvalTarget eval_target
    26: optional list<evaluator.Evaluator> evaluators
    27: optional i64 eval_set_id (api.js_conv='true', go.tag='json:"eval_set_id"')
    28: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    29: optional common.BaseInfo base_info

    30: optional ExptStatistics expt_stats
    31: optional TargetFieldMapping target_field_mapping
    32: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    33: optional common.RuntimeParam target_runtime_param

    40: optional ExptType expt_type
    41: optional i64 max_alive_time
    42: optional SourceType source_type
    43: optional string source_id
    45: optional i32 item_retry_num

    51: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_list // 补充的评估器id+version关联评估器方式，和evaluator_version_ids共同使用，兼容老逻辑

    60: optional ExptTemplateMeta expt_template_meta
    // 评估器得分加权配置
    61: optional ExptScoreWeight score_weight_config
    62: optional bool enable_weighted_score

    // 智能评测相关
    63: optional string thread_id// 关联的智能评测会话ID
    64: optional bool enable_extract_trajectory

    // 触发方式
    70: optional ExptTriggerType trigger_type
    71: optional ExptSource expt_source

    100: optional map<string, string> ext
    // 离线实验分析状态
    101: optional OfflineExptAnalysisStatus offline_expt_analysis_status
}

// 实验模板基础信息
struct ExptTemplateMeta {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')
    3: optional string name
    4: optional string desc
    5: optional ExptType expt_type   // 模板对应的实验类型，当前主要为 Offline
    6: optional Visibility visibility  // 实验模板可见性，默认为空，可见
}

// 实验三元组配置
struct ExptTuple {
    1: optional i64 eval_set_id (api.js_conv='true', go.tag='json:"eval_set_id"')
    2: optional i64 eval_set_version_id (api.js_conv='true', go.tag='json:"eval_set_version_id"')
    3: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    4: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    6: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_items
    7: optional eval_set.EvaluationSet eval_set
    8: optional eval_target.EvalTarget eval_target
    9: optional list<evaluator.Evaluator> evaluators
}

// 实验字段映射和运行时参数配置
struct ExptFieldMapping {
    1: optional TargetFieldMapping target_field_mapping
    2: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    3: optional common.RuntimeParam target_runtime_param
    4: optional i32 item_concur_num
    5: optional i32 item_retry_num
}

// 实验评估器得分加权配置
struct ExptScoreWeight {
    1: optional bool enable_weighted_score
    2: optional map<i64, double> evaluator_score_weights
}

struct ExptTemplate {
    1: optional ExptTemplateMeta meta
    2: optional ExptTuple triple_config
    3: optional ExptFieldMapping field_mapping_config
    4: optional ExptScoreWeight score_weight_config
    5: optional ExptInfo expt_info
    6: optional ExptSource expt_source
    7: optional bool enable_extract_trajectory

    255: optional common.BaseInfo base_info
}

struct TaskTimeRange {
    1: optional i64 start_time (agw.js_conv = "str") // 生效开始时间（时间戳，毫秒）
    2: optional i64 end_time (agw.js_conv = "str") // 生效结束时间（时间戳，毫秒）
}

struct ExptSource {
    1: optional SourceType source_type
    2: optional string source_id

    // 不同source里的源数据结构
    100: optional filter.SpanFilterFields span_filter_fields
    101: optional Scheduler scheduler
    // 采样配置，与 pipeline 节点 task.rule.sampler（见 pipeline.json）及 task.Sampler 对齐
    102: optional task.Sampler sampler
    103: optional TaskTimeRange time_range
}

typedef string Frequency (ts.enum="true")
const Frequency FrequencyEveryday = "every_day"
const Frequency FrequencyMonday = "monday"
const Frequency FrequencyTuesday = "tuesday"
const Frequency FrequencyWednesday = "wednesday"
const Frequency FrequencyThursday = "thursday"
const Frequency FrequencyFriday = "friday"
const Frequency FrequencySaturday = "saturday"
const Frequency FrequencySunday = "sunday"
const Frequency FrequencyEveryHour = "every_hour"
const Frequency FrequencyEveryMinute = "every_minute"

struct Scheduler {
    1: optional bool enabled              // 定时触发器开关，默认关闭
    2: optional Frequency frequency       // 触发频次
    3: optional i64 trigger_at (agw.js_conv = "str")    // 触发时间（时间戳，秒。只使用时间，不使用日期）
    4: optional i64 start_time (agw.js_conv = "str")  // 生效开始时间（时间戳，秒）
    5: optional i64 end_time (agw.js_conv = "str")    // 生效结束时间（时间戳，秒）
    6: optional i32 trigger_interval      // 触发间隔（every_minute时为分钟数，every_hour时为小时数）
}

struct ExptInfo {
    1: optional i64 created_expt_count
    2: optional i64 latest_expt_id (api.js_conv='true', go.tag='json:"latest_expt_id"')
    3: optional ExptStatus latest_expt_status
    4: optional i64 latest_expt_start_time (agw.js_conv = "str") // 最新实验开始时间（时间戳，毫秒）
    5: optional bool cron_activate (go.tag='json:"cron_activate"') // 是否开启定时触发
}

struct TokenUsage {
    1: optional i64 input_tokens (api.js_conv='true', go.tag='json:"input_tokens"')
    2: optional i64 output_tokens (api.js_conv='true', go.tag='json:"output_tokens"')
}

struct ExptStatistics {
    1: optional list<EvaluatorAggregateResult> evaluator_aggregate_results
    2: optional TokenUsage token_usage
    3: optional double credit_cost
    4: optional i32 pending_turn_cnt
    5: optional i32 success_turn_cnt
    6: optional i32 fail_turn_cnt
    7: optional i32 terminated_turn_cnt
    8: optional i32 processing_turn_cnt
}

struct EvaluatorFmtResult {
    1: optional string name
    2: optional double score
}

const string PromptUserQueryFieldKey = "builtin_prompt_user_query"

struct TargetFieldMapping {
    1: optional list<FieldMapping> from_eval_set
}

struct EvaluatorFieldMapping {
    1: required i64 evaluator_version_id (api.js_conv='true', go.tag='json:"evaluator_version_id"')
    2: optional list<FieldMapping> from_eval_set
    3: optional list<FieldMapping> from_target
    4: optional evaluator.EvaluatorIDVersionItem evaluator_id_version_item
}

struct FieldMapping {
    1: optional string field_name
    2: optional string const_value
    3: optional string from_field_name
}

struct ExptFilterOption {
    1: optional string fuzzy_name
    10: optional Filters filters
}

enum ExptRetryMode {
    Unknown = 0
    RetryAll = 1
    RetryFailure = 2
    RetryTargetItems = 3
}

enum ItemRunState {
  Unknown = -1;
  Queueing = 0;  // Queuing
  Processing = 1; // Processing
  Success = 2;    // Success
  Fail = 3;       // Failure
  Terminal = 5;   // Terminated
}

enum TurnRunState {
    Queueing     = 0 // Not started
    Success      = 1 // Execution succeeded
    Fail         = 2 // Execution failed
    Processing   = 3 // In progress
    Terminal     = 4 // Terminated
}

struct ItemSystemInfo {
    1: optional ItemRunState run_state
    2: optional string log_id
    3: optional RunError error
}

struct ExptColumnEvaluator {
    1: required i64 experiment_id (api.js_conv='true', go.tag='json:"experiment_id"')
    2: optional list<ColumnEvaluator> column_evaluators
}

struct ColumnEvaluator {
    1: required i64 evaluator_version_id (api.js_conv='true', go.tag='json:"evaluator_version_id"')
    2: required i64 evaluator_id (api.js_conv='true', go.tag='json:"evaluator_id"')
    3: required evaluator.EvaluatorType evaluator_type
    4: optional string name
    5: optional string version
    6: optional string description
    7: optional bool builtin
}

struct ExptColumnEvalTarget {
    1: optional i64 experiment_id (api.js_conv='true', go.tag='json:"experiment_id"')
    2: optional list<ColumnEvalTarget> column_eval_targets
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
    6: optional dataset.SchemaKey schema_key
}

struct ColumnEvalSetField {
    1: optional string key
    2: optional string name
    3: optional string description
    4: optional common.ContentType content_type
//    5: optional datasetv3.FieldDisplayFormat DefaultDisplayFormat
    6: optional string text_schema
    7: optional dataset.SchemaKey schema_key
}

struct ItemResult {
    1: required i64 item_id (api.js_conv='true', go.tag='json:"item_id"')
    // row粒度实验结果详情
    2: optional list<TurnResult> turn_results
    3: optional ItemSystemInfo system_info
    4: optional i64 item_index (api.js_conv='true', go.tag='json:"item_index"')

    5: optional map<string, string> ext
}

// 行级结果 可能包含多个实验
struct TurnResult {
    1: i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    // 参与对比的实验序列，对于单报告序列长度为1
    2: optional list<ExperimentResult> experiment_results
    3: optional i64 turn_index (api.js_conv='true', go.tag='json:"turn_index"')
}

struct ExperimentResult {
    1: required i64 experiment_id (api.js_conv='true', go.tag='json:"experiment_id"')
    2: optional ExperimentTurnPayload payload
}

struct TurnSystemInfo {
    1: optional TurnRunState turn_run_state
    2: optional string log_id
    3: optional RunError error
}

struct RunError {
    1: required i64 code (api.js_conv='true', go.tag='json:"code"')
    2: optional string message
    3: optional string detail
}

struct TurnEvalSet {
    1: eval_set.Turn turn
}

struct TurnTargetOutput {
    1: optional eval_target.EvalTargetRecord eval_target_record
}

struct TurnEvaluatorOutput {
    1: map<i64, evaluator.EvaluatorRecord> evaluator_records (go.tag = 'json:"evaluator_records"')

    11: optional double weighted_score (go.tag = 'json:"weighted_score"') // 加权汇总得分
}

struct TurnAnnotateResult {
    1: map<i64, AnnotateRecord> annotate_records (go.tag = 'json:"annotate_records"') // tag_key_id -> annotate_record
}

struct AnnotateRecord {
    1: optional i64 annotate_record_id   (api.js_conv = 'true', go.tag = 'json:"annotate_record_id"')
    2: optional i64 tag_key_id (api.js_conv = 'true', go.tag = 'json:"tag_key_id"') // 标签ID
    3: optional string score
    4: optional string boolean_option
    5: optional string categorical_option
    6: optional string  plain_text
    7: optional tag.TagContentType    tag_content_type
    8: optional i64 tag_value_id (api.js_conv = 'true', go.tag = 'json:"tag_value_id"') // 标签选项值ID
}

// 实际行级payload
struct ExperimentTurnPayload {
    1: i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    // 评测数据集数据
    2: optional TurnEvalSet eval_set
    // 评测对象结果
    3: optional TurnTargetOutput target_output
    // 评测规则执行结果
    4: optional TurnEvaluatorOutput evaluator_output
    // 评测系统相关数据日志、error
    5: optional TurnSystemInfo system_info
    // 人工标注结果结果
    6: optional TurnAnnotateResult annotate_result
    // 轨迹分析结果
    7: optional TrajectoryAnalysisResult trajectory_analysis_result
}

struct TrajectoryAnalysisResult {
    1: optional i64 record_id (api.js_conv = 'true', go.tag = 'json:"record_id"')
    2: optional InsightAnalysisStatus Status
}

struct KeywordSearch {
    1: optional string keyword
    2: optional list<FilterField> filter_fields
}

struct ExperimentFilter {
    1: optional Filters filters
    2: optional KeywordSearch keyword_search
}

// 实验模板筛选器，字段设计复用实验的 Filters / KeywordSearch 能力
struct ExperimentTemplateFilter {
    1: optional Filters filters
    2: optional KeywordSearch keyword_search
}

struct Filters {
    1: optional list<FilterCondition> filter_conditions
    2: optional FilterLogicOp logic_op
}

enum FilterLogicOp {
    Unknown = 0
    And = 1
    Or = 2
}

struct FilterField {
    1: required FieldType field_type
    2: optional string field_key // 二级key放此字段里
}

enum FieldType {
    Unknown = 0
    EvaluatorScore = 1    // 评估器得分, FieldKey为evaluatorVersionID,value为score
    CreatorBy = 2
    ExptStatus = 3
    TurnRunState = 4
    TargetID = 5
    EvalSetID = 6
    EvaluatorID = 7
    TargetType = 8
    SourceTarget = 9

    EvaluatorVersionID = 20
    TargetVersionID = 21
    EvalSetVersionID = 22

    ExptType = 30
    SourceType = 31
    SourceID = 32

    KeywordSearch = 41
    EvalSetColumn = 42 // 使用二级key，column_key
    Annotation = 43 // 使用二级key, Annotation_key（具体参考人工标注设计）
    ActualOutput = 44 // 使用二级key，目前使用固定key：content
    EvaluatorScoreCorrected = 45
    Evaluator = 46 // 使用二级key，evaluator_version_id
    ItemID = 47
    ItemRunState = 48
    AnnotationScore = 49 // 使用二级key, field_key为tag_key_id, value为score
    AnnotationText = 50 // 使用二级key, field_key为tag_key_id, value为文本
    AnnotationCategorical = 51  // 使用二级key, field_key为tag_key_id, value为tag_value_id

    TotalLatency = 60 // 目前使用固定key：total_latency
    InputTokens = 61 // 目前使用固定key：input_tokens
    OutputTokens = 62 // 目前使用固定key：output_tokens
    TotalTokens = 63 // 目前使用固定key：total_tokens

    ExperimentTemplateID = 70
    EvaluatorWeightedScore = 71
    UpdatedBy = 72
    CronActivate = 73
    TriggerType = 74
}

// 字段过滤器
struct FilterCondition {
    // 过滤字段，比如评估器ID
    1: FilterField field
    // 操作符，比如等于、包含、大于、小于等
    2: FilterOperatorType operator
    // 操作值;支持多种类型的操作值；
    3: string value
    4: optional SourceTarget source_target
}

struct SourceTarget {
    1: optional eval_target.EvalTargetType eval_target_type
    3: optional list<string> source_target_ids
}

enum FilterOperatorType {
    Unknown = 0
    Equal = 1 // 等于
    NotEqual = 2    // 不等于
    Greater = 3        // 大于
    GreaterOrEqual = 4 // 大于等于
    Less = 5        // 小于
    LessOrEqual = 6 // 小于等于
    In = 7 // 包含
    NotIn = 8 // 不包含
    Like = 9 // 全文搜索
    NotLike = 10 // 全文搜索反选
    IsNull = 11 // 为空
    IsNotNull = 12 //非空

}

enum ExptAggregateCalculateStatus {
    Unknown = 0
    Idle = 1
    Calculating = 2
}

// 实验粒度聚合结果
struct ExptAggregateResult {
    1: required i64 experiment_id (api.js_conv = 'true', go.tag = 'json:"experiment_id"')
    2: optional map<i64, EvaluatorAggregateResult> evaluator_results (go.tag = 'json:"evaluator_results"')
    3: optional ExptAggregateCalculateStatus status
    4: optional map<i64, AnnotationAggregateResult> annotation_results (go.tag = 'json:"annotation_results"')    // tag_key_id -> result
    5: optional EvalTargetAggregateResult eval_target_aggr_result
    6: optional i64 update_time // timestamp in seconds

    10: optional list<AggregatorResult> weighted_results (go.tag = 'json:"weighted_results"')
}

struct EvalTargetAggregateResult {
    1: optional i64 target_id (api.js_conv = 'true', go.tag = 'json:"target_id"')
    2: optional i64 target_version_id (api.js_conv = 'true', go.tag = 'json:"target_version_id"')

    5: optional list<AggregatorResult> latency
    6: optional list<AggregatorResult> input_tokens
    7: optional list<AggregatorResult> output_tokens
    8: optional list<AggregatorResult> total_tokens
}

// 评估器版本粒度聚合结果
struct EvaluatorAggregateResult {
    1: required i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    2: optional list<AggregatorResult> aggregator_results
    3: optional string name
    4: optional string version
}

// 人工标注项粒度聚合结果
struct AnnotationAggregateResult {
    1: required i64 tag_key_id (api.js_conv = 'true', go.tag = 'json:"tag_key_id"')
    2: optional list<AggregatorResult> aggregator_results
    3: optional string name
}

// 一种聚合器类型的聚合结果
struct  AggregatorResult {
    1: required AggregatorType aggregator_type
    2: optional AggregateData data
}

// 聚合器类型
enum AggregatorType {
      Average = 1
      Sum = 2
      Max = 3
      Min = 4
      Distribution = 5; // 得分的分布情况
}

enum DataType {
      Double = 0; // 默认，有小数的浮点数值类型
      ScoreDistribution = 1; // 得分分布
      OptionDistribution = 2    // 选项分布
}

struct ScoreDistribution {
    1: optional list<ScoreDistributionItem> score_distribution_items
}

struct ScoreDistributionItem {
    1: required string score
    2: required i64 count (api.js_conv='true', go.tag='json:"count"')
    3: required double percentage
}

struct AggregateData {
    1: required DataType data_type
    2: optional double value
    3: optional ScoreDistribution score_distribution
    4: optional OptionDistribution option_distribution
}

struct OptionDistribution {
    1: optional list<OptionDistributionItem> option_distribution_items
}

struct OptionDistributionItem {
    1: required string option   // 值为tag_value_id,或`其他`
    2: required i64 count (api.js_conv='true', go.tag='json:"count"')
    3: required double percentage
}

struct ExptStatsInfo {
    1: optional i64 expt_id
    2: optional string source_id
    3: optional ExptStatistics expt_stats
}

struct ExptColumnAnnotation {
    1: required i64 experiment_id (api.js_conv='true', go.tag='json:"experiment_id"')
    2: optional list<ColumnAnnotation> column_annotations
}

// 标签信息，沿用数据基座Tag定义
struct ColumnAnnotation {
    1: optional i64 tag_key_id (api.js_conv="true", go.tag='json:"tag_key_id"')
    2: optional string tag_key_name                         // tag key name
    3: optional string description                          // 描述
    4: optional tag.TagStatus status

    13: optional list<tag.TagValue> tag_values                 // 标签选项值
    14: optional tag.TagContentType content_type                 // 标签内容类型
    15: optional tag.TagContentSpec content_spec                 // 标签内容限制
}

typedef string ExptResultExportType(ts.enum="true")

const ExptResultExportType ExptResultExportType_CSV = "CSV"

typedef string CSVExportStatus(ts.enum="true")

const CSVExportStatus CSVExportStatus_Unknown = "Unknown"
const CSVExportStatus CSVExportStatus_Running = "Running"
const CSVExportStatus CSVExportStatus_Success = "Success"
const CSVExportStatus CSVExportStatus_Failed = "Failed"

struct ExptResultExportRecord {
    1: required i64 export_id (api.js_conv='true', go.tag='json:"export_id"')
    2: required i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: required i64 expt_id (api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: required CSVExportStatus csv_export_status
    5: optional common.BaseInfo base_info
    6: optional i64 start_time (api.js_conv='true', go.tag='json:"start_time"')
    7: optional i64 end_time (api.js_conv='true', go.tag='json:"end_time"')
    // deprecated, cause not match snake name
    8: optional string URL
    9: optional bool expired
    10: optional RunError error
    11: optional string url
}

// 分析任务状态
typedef string InsightAnalysisStatus(ts.enum="true")

const InsightAnalysisStatus InsightAnalysisStatus_Unknown = "Unknown"
const InsightAnalysisStatus InsightAnalysisStatus_Running = "Running"
const InsightAnalysisStatus InsightAnalysisStatus_Success = "Success"
const InsightAnalysisStatus InsightAnalysisStatus_Failed = "Failed"

// 投票类型
typedef string InsightAnalysisReportVoteType(ts.enum="true")

// 未投票
const InsightAnalysisReportVoteType InsightAnalysisReportVoteType_None = "None"
// 点赞
const InsightAnalysisReportVoteType InsightAnalysisReportVoteType_Upvote = "Upvote"
// 点踩
const InsightAnalysisReportVoteType InsightAnalysisReportVoteType_Downvote = "Downvote"

// 洞察分析记录
struct ExptInsightAnalysisRecord {
    1: required i64 record_id (api.js_conv='true', go.tag='json:"record_id"')
    2: required i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: required i64 expt_id (api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: required InsightAnalysisStatus analysis_status
    5: optional i64 analysis_report_id (api.js_conv = 'true', go.tag = 'json:"analysis_report_id"')
    6: optional string analysis_report_content
    7: optional ExptInsightAnalysisFeedback expt_insight_analysis_feedback
    8: optional common.BaseInfo base_info

    21: optional list<ExptInsightAnalysisIndex> analysis_report_index
}

struct ExptInsightAnalysisIndex {
    1: optional string id
    2: optional string title
}

// 洞察分析反馈统计
struct ExptInsightAnalysisFeedback {
    1: optional i32 upvote_cnt
    2: optional i32 downvote_cnt
    // 当前用户点赞状态，用于展示用户是否已点赞点踩
    3: optional InsightAnalysisReportVoteType current_user_vote_type
}

// 洞察分析反馈评论
struct ExptInsightAnalysisFeedbackComment {
    1: required i64 comment_id (api.js_conv='true', go.tag='json:"comment_id"')
    2: required i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: required i64 expt_id (api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: required i64 record_id (api.js_conv='true', go.tag='json:"record_id"')
    5: required string content
    6: optional common.BaseInfo base_info
}

struct ExptInsightAnalysisFeedbackVote {
    1: optional i64 id (api.js_conv='true', go.tag='json:"comment_id"')
    2: optional FeedbackActionType feedback_action_type
}

// 反馈动作
typedef string FeedbackActionType(ts.enum="true")

const FeedbackActionType FeedbackActionType_Upvote = "Upvote"
const FeedbackActionType FeedbackActionType_Cancel_Upvote = "Cancel_Upvote"
const FeedbackActionType FeedbackActionType_Downvote = "Downvote"
const FeedbackActionType FeedbackActionType_Cancel_Downvote = "Cancel_Downvote"
const FeedbackActionType FeedbackActionType_Create_Comment = "Create_Comment"
const FeedbackActionType FeedbackActionType_Update_Comment = "Update_Comment"
const FeedbackActionType FeedbackActionType_Delete_Comment = "Delete_Comment"

