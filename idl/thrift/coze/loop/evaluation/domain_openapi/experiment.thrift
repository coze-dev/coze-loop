namespace go coze.loop.evaluation.domain_openapi.experiment

include "common.thrift"
include "eval_set.thrift"
include "evaluator.thrift"
include "eval_target.thrift"
// data 侧 filter (别名 data_filter, 与 observability filter 区分以便 BAM/thriftgo 无歧义解析);
// item 圈选复用同一 Filter/FilterField, 与内部 domain expt.EvalSetConfig.item_filter 同型透传。
include "../../data/domain/data_filter.thrift"

// 实验状态
typedef string ExperimentStatus (ts.enum = "true")
const ExperimentStatus ExperimentStatus_Pending = "pending"
const ExperimentStatus ExperimentStatus_Processing = "processing"
const ExperimentStatus ExperimentStatus_Success = "success"
const ExperimentStatus ExperimentStatus_Failed = "failed"
const ExperimentStatus ExperimentStatus_Terminated = "terminated"
const ExperimentStatus ExperimentStatus_SystemTerminated = "system_terminated"
const ExperimentStatus ExperimentStatus_Draining = "draining"

// 实验类型
typedef string ExperimentType (ts.enum = "true")
const ExperimentType ExperimentType_Offline = "offline"
const ExperimentType ExperimentType_Online = "online"

// 离线实验分析状态（OpenAPI 字符串枚举，与 domain OfflineExptAnalysisStatus 对应）
typedef string OfflineExptAnalysisStatus (ts.enum = "true")
const OfflineExptAnalysisStatus OfflineExptAnalysisStatus_NotStarted = "not_started"
const OfflineExptAnalysisStatus OfflineExptAnalysisStatus_Processing = "processing"
const OfflineExptAnalysisStatus OfflineExptAnalysisStatus_Success = "success"
const OfflineExptAnalysisStatus OfflineExptAnalysisStatus_Failed = "failed"
const OfflineExptAnalysisStatus OfflineExptAnalysisStatus_Superseded = "superseded"

// 聚合器类型
typedef string AggregatorType (ts.enum = "true")
const AggregatorType AggregatorType_Average = "average"
const AggregatorType AggregatorType_Sum = "sum"
const AggregatorType AggregatorType_Max = "max"
const AggregatorType AggregatorType_Min = "min"
const AggregatorType AggregatorType_Distribution = "distribution"

// 数据类型
typedef string DataType (ts.enum = "true")
const DataType DataType_Double = "double"
const DataType DataType_ScoreDistribution = "score_distribution"

typedef string ItemRunState (ts.enum = "true")
const ItemRunState ItemRunState_Queueing = "queueing"
const ItemRunState ItemRunState_Processing = "processing"
const ItemRunState ItemRunState_Success = "success"
const ItemRunState ItemRunState_Fail = "fail"
const ItemRunState ItemRunState_Terminal = "terminal"


typedef string TurnRunState (ts.enum = "true")
const TurnRunState TurnRunState_Queueing = "queueing"
const TurnRunState TurnRunState_Processing = "processing"
const TurnRunState TurnRunState_Success = "success"
const TurnRunState TurnRunState_Fail = "fail"
const TurnRunState TurnRunState_Terminal = "terminal"

typedef string ExptRetryMode (ts.enum = "true")
const ExptRetryMode ExptRetryMode_RetryAll = "retry_all"
const ExptRetryMode ExptRetryMode_RetryFailure = "retry_failure"
const ExptRetryMode ExptRetryMode_RetryTargetItems = "retry_target_items"

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
    1: optional i64 evaluator_id (api.js_conv = "true", go.tag = 'json:"evaluator_id"')
    2: optional string version
    3: optional list<FieldMapping> from_eval_set
    4: optional list<FieldMapping> from_target
}

// ===== item-centric 多评测集配置 (OpenAPI 版本字符串风格) =====
// 与内部 expt.EvalSetConfig 对应; OpenAPI 用 id + 版本字符串, handler 解析成内部 version_id.
// 非空 = 走新建模路径 (多评测集 + 每集 evaluator/target 绑定); 缺省则走老的单评测集形态.

// per-set 的一个 evaluator binding (版本字符串风格)
struct OpenAPIExptEvaluatorConf {
    1: optional i64 evaluator_id (api.js_conv = "true", go.tag = 'json:"evaluator_id"')
    2: optional string version                       // 评估器版本字符串, handler 解析成 evaluator_version_id
    3: optional string alias                         // 多实例区分(judge_A/judge_B); 缺省 '' 默认实例
    10: optional list<FieldMapping> from_eval_set    // 评测集字段 → evaluator 输入
    11: optional list<FieldMapping> from_target      // target 输出 → evaluator 输入
    20: optional common.RuntimeParam runtime_param   // alias 多实例核心动机: 同 version 不同参数
    30: optional double score_weight                 // enable_weighted_score 开启时参与加权
    40: optional data_filter.Filter filter           // 行级过滤: 命中才执行本 binding (与内部 ExptEvaluatorConf.filter 同型)
    41: optional i32 filter_mode                      // 0 None / 1 Include / 2 Exclude
}

// per-set target 运行配置; 本期 len<=1
// target_id 不传时继承 request 顶层 eval_target_param;
// 跨空间多集场景 per-set 需显式指定 target_id 以对该 set 的评测对象做来源空间授权(执行仍用顶层 target)。
struct OpenAPIExptTargetConf {
    1: optional i64 target_id (api.js_conv = "true", go.tag = 'json:"target_id"')   // per-set 评测对象 id; 跨空间授权必需
    10: optional TargetFieldMapping field_mapping    // 本评测集字段 → target 输入
    20: optional common.RuntimeParam runtime_param
}

// 一个评测集 + 该集的完整配置包 (版本字符串风格)
struct OpenAPIEvalSetConfig {
    1: optional i64 eval_set_id (api.js_conv = "true", go.tag = 'json:"eval_set_id"')
    2: optional string eval_set_version              // 版本字符串, handler 解析成 eval_set_version_id (锁定版本)
    10: optional list<OpenAPIExptEvaluatorConf> evaluator_confs // (evaluator_version_id, alias) 在 set 内唯一
    20: optional list<OpenAPIExptTargetConf> target_confs      // 本期 len<=1; 不传=继承顶层 target
    // 题目圈选: 不传=全集; 点选=item_id in [...]; 条件圈选=tag 条件 (复用 data data_filter.Filter, 与内部 EvalSetConfig.item_filter 同型透传)
    // 校验白名单(应用层, 与内部一致): query_type ∈ {eq,not_eq,in,not_in}; 单层不嵌套(sub_filter 必空); field_name ∈ {item_id, tag key}; field_type ∈ {long, tag}
    30: optional data_filter.Filter item_filter

    40: optional common.SharedResourceOption shared_option        // 跨空间: 该 set 评测集来源空间; nil/!is_shared=同空间
    41: optional common.SharedResourceOption target_shared_option // 跨空间: 该 set 评测对象来源空间
}

// 实验评测集来源模式 (OpenAPI 字符串枚举, 与 domain ExptEvalSetSourceType 对应)
// 读接口分流: single_set=老实验(单评测集) / multi_set_config=新实验(多评测集)
typedef string ExptEvalSetSourceType (ts.enum = "true")
const ExptEvalSetSourceType ExptEvalSetSourceType_SingleSet = "single_set"
const ExptEvalSetSourceType ExptEvalSetSourceType_MultiSetConfig = "multi_set_config"

// ===== 实验级跑法配置 (SUA / run_mode) OpenAPI 版本 =====
// domain 侧对应 domain/expt.thrift 的 ExptRunMode/SuaMode(int enum) + RunModeConfig。
// 与既有 ExptEvalSetSourceType 一致, OpenAPI 侧用字符串枚举对等映射, 由 convertor 与内部 int 枚举互转,
// 避免 include domain/expt.thrift 引入与本文件已定义结构 (如 TargetFieldMapping/ExptNotificationConf) 的同名符号冲突。

// ExptRunMode 实验评测模式(跑法), 对齐 domain ExptRunMode: single/fixed_script/sua/goal。
typedef string ExptRunMode (ts.enum = "true")
const ExptRunMode ExptRunMode_SingleTurn = "single_turn"                       // 单轮
const ExptRunMode ExptRunMode_FixedScriptMultiTurn = "fixed_script_multi_turn" // 固定脚本多轮
const ExptRunMode ExptRunMode_SuaMultiTurn = "sua_multi_turn"                  // SUA 驱动多轮
const ExptRunMode ExptRunMode_Goal = "goal"                                    // 目标驱动

// SuaMode 模拟用户(SUA)生成下一轮 query 的模式, 对齐 domain SuaMode。
typedef string SuaMode (ts.enum = "true")
const SuaMode SuaMode_HumanLoop = "human_loop" // LLM 按人设驱动
const SuaMode SuaMode_Loop = "loop"            // 上轮 eval 结果透传成下一轮
const SuaMode SuaMode_Fixed = "fixed"          // 照固定脚本

// RunModeConfig 实验级跑法配置 (OpenAPI 版本, 对齐 domain RunModeConfig)。run_mode 是顶层跑法总开关;
// sua_mode 是 SUA 专属子字段, 仅 run_mode ∈ {sua_multi_turn, goal} 时生效。
// 仅 SandboxAgent 评测对象 + MultiSetConfig 实验生效。
//
// **SUA 模型不是调用方参数**: 由平台 TCC 统一控制(含密钥)。字段 4 sua_model_id **已移除**,
// sua_model_name 已弃用(仅调试)。详见 domain/expt.thrift 同名 struct 注释。
//
// 字段号与 domain 版**逐一对齐** (含 4 号的保留, 6-10 为两级配置的实验级一半: SUA 行为四项
// + max_turns); 合并规则「题目级优先、实验级兜底」详见 domain/expt.thrift 的同名 struct 注释。
struct RunModeConfig {
    1: optional ExptRunMode run_mode (go.tag = 'json:"run_mode"')
    2: optional i32 max_run_minutes (go.tag = 'json:"max_run_minutes"')
    3: optional SuaMode sua_mode (go.tag = 'json:"sua_mode"')
    // 4 号**永久保留**: 曾是 sua_model_id, 与 domain 版同步移除。号段不复用。
    // SUA 模型名, **已弃用, 仅调试用**; 常规路径不传, 由平台 TCC 选模型并带密钥。
    5: optional string sua_model_name (go.tag = 'json:"sua_model_name"')
    // sua_goal 模拟用户要达成的目标 (SUA 据此判断"任务是否完成")。
    6: optional string sua_goal (go.tag = 'json:"sua_goal"')
    // sua_persona 模拟用户人设。human_loop 跑法必需 —— sua-cli 缺它报 INVALID_CONFIG。
    7: optional string sua_persona (go.tag = 'json:"sua_persona"')
    // sua_behavioral_constraints 模拟用户的行为约束 (如"每轮只追问一个点""不泄露参考答案")。
    8: optional string sua_behavioral_constraints (go.tag = 'json:"sua_behavioral_constraints"')
    // sua_pe_template loop 跑法必需的 PE 模板, **必须含 {{eval_result}} 占位符**;
    // 缺它 loop 直接 INVALID_CONFIG。
    9: optional string sua_pe_template (go.tag = 'json:"sua_pe_template"')
    // max_turns 实验级轮数上限 (题目级同名字段在 ItemRunConf, 题目级优先)。
    10: optional i32 max_turns (go.tag = 'json:"max_turns"')
    // skills_mode SandboxAgent 跑法的技能模式, 原样透传到 case-file experiment_info.skills_mode。
    // 合法值 merge / disable_test_case; 非法值在 convertor 报 CommonInvalidParamCode。
    11: optional string skills_mode (go.tag = 'json:"skills_mode"')
}

// per-set 运行期增量信息 (纯读模型; Get 全填含详情, List 只填 id/count)
struct ExptEvalSetDetail {
    1: optional i64 eval_set_id (api.js_conv = "true", go.tag = 'json:"eval_set_id"')
    2: optional i64 eval_set_version_id (api.js_conv = "true", go.tag = 'json:"eval_set_version_id"')
    3: optional bool is_primary                      // 主集(封面), 与 experiment.eval_set_id 一致
    4: optional i32 item_count                       // 该 set 选入实验的 item 数; 首跑前为 0
    5: optional eval_set.EvaluationSet eval_set      // Get 填充详情; List 不填
    6: optional string dataset_key                   // 评测集业务唯一键; 便于 GetExperiment 直接展示/定位
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
    // alias 多实例别名 (default/judge_b 等); 同 version 多实例时区分, 老数据为空串。
    21: optional string alias
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
    2: optional i64 count (api.js_conv = 'true', go.tag = 'json:"count"')
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
//
// run_mode_config (115) 是**跑法配置的读侧回显**, 2026-08 补齐。此前只有写侧
// (SubmitExperimentRequest 47 号字段) 能配跑法, 读侧无字段 —— OpenAPI 用户提交后
// 查不到自己配了什么跑法, 而内部接口 (domain/expt.thrift 的 115 号字段) 一直能回显,
// 即"内部能看、OpenAPI 看不到"的不对称。
//
// 注意本文件用的是 domain_openapi 自己那套**字符串枚举**结构 (本文件的 struct
// RunModeConfig, 与 ExptEvalSetSourceType 同套模式), 不要 include domain/expt.thrift ——
// 会符号冲突。两套枚举的整数编号刻意不同 (见 domain/expt.thrift 的 ExptRunMode 注释),
// 所以跨模型搬字段时必须过一次显式转换, 不能直接赋值。
struct Experiment {
    // 基本信息
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional string name
    3: optional string description
    4: optional string experiment_group_key

    // 运行信息
    10: optional ExperimentStatus status // 实验状态
    11: optional i64 started_at (api.js_conv = 'true', go.tag = 'json:"started_at"') // ISO 8601格式
    12: optional i64 ended_at (api.js_conv = 'true', go.tag = 'json:"ended_at"') // ISO 8601格式
    13: optional i32 item_concur_num // 评测集并发数
    14: optional common.RuntimeParam target_runtime_param   // 运行时参数
    15: optional i32 item_retry_num // 单条数据失败重试次数

    // 三元组信息
    31: optional TargetFieldMapping target_field_mapping
    32: optional list<EvaluatorFieldMapping> evaluator_field_mapping
    33: optional eval_set.EvaluationSet eval_set
    34: optional eval_target.EvalTarget eval_target
    // 评估器 id + version 关联（与 evaluator_field_mapping 共同使用，兼容老逻辑）
    35: optional list<evaluator.EvaluatorIDVersionItem> evaluator_id_version_list (go.tag = 'json:"evaluator_id_version_list"')

    // 统计信息
    50: optional ExperimentStatistics expt_stats

    60: optional bool enable_extract_trajectory
    // 实验模板基础信息
    62: optional ExptTemplateMeta expt_template_meta

    // 离线实验分析状态
    61: optional OfflineExptAnalysisStatus offline_expt_analysis_status

    // 通知配置
    70: optional ExptNotificationConf notification_conf
    // ★ 多评测集读视图 (与 domain Experiment 110~114 同义)
    110: optional ExptEvalSetSourceType eval_set_source_type // single_set(老) / multi_set_config(新); 读接口分流开关
    111: optional list<OpenAPIEvalSetConfig> eval_set_configs // 权威配置回显, 与 Create OApi 入参同构
    112: optional list<ExptEvalSetDetail> eval_set_details    // per-set 评测集详情 + item 数 (Get 全填; List 只 id/count)
    113: optional i32 evaluators_concur_num                   // 评估器并发数回显
    114: optional i64 total_item_count (api.js_conv = 'true', go.tag = 'json:"total_item_count"') // 实验绑定 item 总数; 首跑前为 0
    // 跑法配置回显。字段号 115 与 domain/expt.thrift 的 struct Experiment 刻意对齐, 便于两套
    // 读模型对照 —— 它们是同一个概念的两种表示 (本文件用字符串枚举, domain 用整数枚举)。
    // 用本文件已有的 RunModeConfig (字符串枚举), 不要 include domain/expt.thrift —— 会符号冲突。
    115: optional RunModeConfig run_mode_config

    // ★ 中心化调度读视图。字段号 116~118 同样与 domain/expt.thrift 对齐。
    // 调度优先级 (1-99, 越大越优先); 历史数据为 1。
    // 注: 它只影响 scheduler_mode=enforce 的实验; legacy 实验此值虽有 (默认 1) 但不参与调度排序。
    116: optional i32 priority_level
    // 执行模式: legacy(旧 per-experiment 链路) / enforce(中心调度)。
    // **legacy 实验也回显**: "为什么我的实验没进中心调度"是最高频疑问, 回显 legacy 可一眼确认。
    117: optional string scheduler_mode
    // 单 item 预期资源消耗向量; "有则回显、无则省略"(legacy 实验确实没申报)。
    118: optional ExpectedQuotaConsumption expected_quota_consumption

    // 注: scheduler_scope **不进读模型**。它是不透明调度域 ID, 对调用方无可用语义却泄露部署拓扑;
    // 内部运维需要时直接查 experiment.scheduler_scope 列。


    100: optional common.BaseInfo base_info
}

// 单资源预期消耗。与 domain/expt.thrift 的 ExpectedResourceConsumption 同构 ——
// 本文件另立一份而非 include, 与 RunModeConfig / ExptEvalSetSourceType 同套模式, 避免符号冲突。
struct ExpectedResourceConsumption {
    // 资源类别: sandbox / agent_account / model / evaluator
    1: optional string category
    // 资源标识: default / doubao_pro / gpt5.5 等
    2: optional string resource_key
    // 单 item 预期占用量; 单位由服务端 TCC 资源配置定义, 不由调用方指定
    3: optional i64 amount (api.js_conv = 'true', go.tag = 'json:"amount"')
}

// 单 item 的多资源预期消耗向量。
struct ExpectedQuotaConsumption {
    1: optional list<ExpectedResourceConsumption> resources
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
    1: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    2: optional i64 evaluator_id (api.js_conv = 'true', go.tag = 'json:"evaluator_id"')
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

    10: optional double weighted_score // 评估器加权得分

    20: optional TurnSystemInfo system_info
}

struct TurnSystemInfo {
    1: optional TurnRunState turn_run_state
    2: optional string log_id
    3: optional RunError error
}

// 轮次结果
struct TurnResult {
    1: optional string turn_id (api.js_conv = 'true', go.tag = 'json:"turn_id"')
    2: optional ResultPayload payload
}

// 数据项结果
struct ItemResult {
    1: optional i64 item_id (api.js_conv = 'true', go.tag = 'json:"item_id"')   // 数据项(行)ID
    2: optional list<TurnResult> turn_results   // 轮次结果，单轮仅有一个元素

    20: optional ItemSystemInfo system_info
}

struct ItemSystemInfo {
    1: optional ItemRunState run_state
    2: optional string log_id
    3: optional RunError error
    4: optional i32 total_runs // 该 item 在最新一轮运行内评测对象的调用次数（含系统自动重试）
    5: optional list<RetryRecord> retry_records // 历次调用明细（含自动重试），供详情逐次查看 replay 等日志
}

// 评测对象历次调用记录（每次调用/自动重试一条）
struct RetryRecord {
    1: optional i64 record_id (api.js_conv='true', go.tag='json:"record_id"')
    2: optional string trace_id
    3: optional i32 status
    4: optional i64 created_at (api.js_conv='true', go.tag='json:"created_at"')
    5: optional bool is_final
    6: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"')
    7: optional string fornax_sandbox_log_url // 沙箱日志 URL 集合(JSON 串)：{orchestrator,agent,replay}
}

// ===============================
// 实验模板相关结构定义
// ===============================

// 实验模板基础信息
struct ExptTemplateMeta {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional string name
    4: optional string description
    5: optional ExperimentType expt_type   // 模板对应的实验类型，当前主要为 Offline
}

// 实验三元组配置
struct ExptTuple {
    1: optional i64 eval_set_id (api.js_conv = 'true', go.tag = 'json:"eval_set_id"')
    2: optional i64 eval_set_version_id (api.js_conv = 'true', go.tag = 'json:"eval_set_version_id"')
    3: optional i64 target_id (api.js_conv = 'true', go.tag = 'json:"target_id"')
    4: optional i64 target_version_id (api.js_conv = 'true', go.tag = 'json:"target_version_id"')
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
    5: optional i32 item_retry_num
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
    5: optional bool enable_extract_trajectory

    // 通知配置
    10: optional ExptNotificationConf notification_conf

    100: optional common.BaseInfo base_info
}

// ===============================
// 筛选能力结构（与 domain/expt.thrift 结构一致）
// ===============================

// 筛选逻辑操作符（对应 domain/expt FilterLogicOp）
typedef string FilterLogicOp (ts.enum = "true")
const FilterLogicOp FilterLogicOp_Unknown = "unknown"
const FilterLogicOp FilterLogicOp_And = "and"
const FilterLogicOp FilterLogicOp_Or = "or"

// 筛选操作符类型（对应 domain/expt FilterOperatorType）
typedef string FilterOperatorType (ts.enum = "true")
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
typedef string FilterFieldType (ts.enum = "true")
const FilterFieldType FilterFieldType_Unknown = "unknown"
const FilterFieldType FilterFieldType_EvaluatorScore = "evaluator_score"
const FilterFieldType FilterFieldType_CreatorBy = "creator_by"
const FilterFieldType FilterFieldType_ExptStatus = "expt_status"
const FilterFieldType FilterFieldType_TurnRunState = "turn_run_state"
const FilterFieldType FilterFieldType_UpdatedBy = "updated_by"
const FilterFieldType FilterFieldType_EvalSetID = "eval_set_id"
const FilterFieldType FilterFieldType_TargetID = "target_id"
const FilterFieldType FilterFieldType_EvaluatorID = "evaluator_id"
const FilterFieldType FilterFieldType_TargetType = "target_type"
const FilterFieldType FilterFieldType_SourceTarget = "source_target"
const FilterFieldType FilterFieldType_EvaluatorVersionID = "evaluator_version_id"
const FilterFieldType FilterFieldType_TargetVersionID = "target_version_id"
const FilterFieldType FilterFieldType_EvalSetVersionID = "eval_set_version_id"
const FilterFieldType FilterFieldType_ExptType = "expt_type"
const FilterFieldType FilterFieldType_SourceType = "source_type"
const FilterFieldType FilterFieldType_SourceID = "source_id"
const FilterFieldType FilterFieldType_KeywordSearch = "keyword_search"
const FilterFieldType FilterFieldType_EvalSetColumn = "eval_set_column"
const FilterFieldType FilterFieldType_Annotation = "annotation"
const FilterFieldType FilterFieldType_ActualOutput = "actual_output"
const FilterFieldType FilterFieldType_EvaluatorScoreCorrected = "evaluator_score_corrected"
const FilterFieldType FilterFieldType_Evaluator = "evaluator"
const FilterFieldType FilterFieldType_ItemID = "item_id"
const FilterFieldType FilterFieldType_ItemRunState = "item_run_state"
const FilterFieldType FilterFieldType_AnnotationScore = "annotation_score"
const FilterFieldType FilterFieldType_AnnotationText = "annotation_text"
const FilterFieldType FilterFieldType_AnnotationCategorical = "annotation_categorical"
const FilterFieldType FilterFieldType_TotalLatency = "total_latency"
const FilterFieldType FilterFieldType_InputTokens = "input_tokens"
const FilterFieldType FilterFieldType_OutputTokens = "output_tokens"
const FilterFieldType FilterFieldType_TotalTokens = "total_tokens"
const FilterFieldType FilterFieldType_ExperimentTemplateID = "experiment_template_id"
const FilterFieldType FilterFieldType_EvaluatorWeightedScore = "evaluator_weighted_score"
const FilterFieldType FilterFieldType_CronActivate = "cron_activate"
const FilterFieldType FilterFieldType_TriggerType = "trigger_type"
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
    // 与 domain/expt.SourceTarget 对齐，兼容 source_target 复合筛选入参（field_type=source_target）
    4: optional SourceTarget source_target
}

// source_target 复合筛选参数（对应 domain/expt SourceTarget）
struct SourceTarget {
    1: optional eval_target.EvalTargetType eval_target_type
    3: optional list<string> source_target_ids
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

// 实验列表筛选（对应 domain/expt ExptFilterOption）
struct ExperimentFilterOption {
    1: optional string fuzzy_name
    2: optional list<ExptEvalSetSourceType> eval_set_source_types // 评测集来源模式筛选 (与 fuzzy_name 同级, 不走 filters); 未传默认排除 multi_set_config
    10: optional Filters filters
}

// 实验报告顶层筛选结构
struct ExperimentResultFilter {
    1: optional Filters filters
    2: optional KeywordSearch keyword_search
}

// ===============================
// 实验报告导出相关结构（对应 domain/expt ExptResultExportType / ExptResultExportRecord）
// ===============================

// 实验报告导出类型
typedef string ExptResultExportType (ts.enum = "true")
const ExptResultExportType ExptResultExportType_CSV = "CSV"

// CSV 导出任务状态
typedef string CSVExportStatus (ts.enum = "true")
const CSVExportStatus CSVExportStatus_Unknown = "Unknown"
const CSVExportStatus CSVExportStatus_Running = "Running"
const CSVExportStatus CSVExportStatus_Success = "Success"
const CSVExportStatus CSVExportStatus_Failed = "Failed"

// 运行错误（对应 domain/expt RunError）
struct RunError {
    1: optional i64 code (api.js_conv = 'true', go.tag = 'json:"code"')
    2: optional string message
    3: optional string detail
}

// 实验报告导出列规格（对应 domain ExptResultExportColumnSpec）
struct ExptResultExportColumnSpec {
    // 评测集字段：ColumnEvalSetField.Key
    1: optional list<string> eval_set_fields (go.tag = 'json:"eval_set_fields"')
    // 评测对象输出（非性能指标）：ColumnEvalTarget.Name，如 actual_output、trajectory、自定义输出名
    2: optional list<string> eval_target_outputs (go.tag = 'json:"eval_target_outputs"')
    // 性能指标：ColumnEvalTarget.Name（如 eval_target_total_latency、eval_target_input_tokens 等）
    3: optional list<string> metrics (go.tag = 'json:"metrics"')
    // 评估器版本 ID 列表；每个 ID 导出该评估器的 score 与 reason 列
    4: optional list<i64> evaluator_version_ids (api.js_conv = 'true', go.tag = 'json:"evaluator_version_ids"')
    // 是否导出加权分数
    5: optional bool weighted_score (go.tag = 'json:"weighted_score"')
    // 人工标注：每项为标注 TagKeyID，与 ColumnAnnotation.TagKeyID 对应，导出该标注列
    6: optional list<i64> tag_key_ids (api.js_conv = 'true', go.tag = 'json:"tag_key_ids"')
}

// 实验报告导出记录（对应 domain/expt ExptResultExportRecord）
struct ExptResultExportRecord {
    1: optional i64 export_id (api.js_conv = 'true', go.tag = 'json:"export_id"')
    2: optional i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional i64 expt_id (api.js_conv = 'true', go.tag = 'json:"expt_id"')
    4: optional CSVExportStatus csv_export_status
    5: optional common.BaseInfo base_info
    6: optional i64 start_time (api.js_conv = 'true', go.tag = 'json:"start_time"')
    7: optional i64 end_time (api.js_conv = 'true', go.tag = 'json:"end_time"')
    8: optional bool expired
    9: optional RunError error
    10: optional string url
}

// ===============================
// 通知配置相关结构定义
// ===============================

// 通知配置（公共触发条件 + 各渠道独立开关/参数）
struct ExptNotificationConf {
    // 公共触发条件（统一，前端只需配一份 filter）
    1: optional Filters filter
    // Webhook 渠道配置
    10: optional WebhookNotificationConf webhook
    // 飞书渠道配置
    11: optional FeishuNotificationConf feishu_notification
}

struct WebhookNotificationConf {
    1: optional bool enable
    2: optional string urls             // Webhook URL 列表，多个用逗号分隔
}

struct FeishuNotificationConf {
    1: optional bool enable
    2: optional string user_id          // 通知目标用户 ID（为空时默认用实验创建者）
}
