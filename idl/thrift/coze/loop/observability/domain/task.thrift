namespace go coze.loop.observability.domain.task

include "common.thrift"
include "filter.thrift"
include "export_dataset.thrift"

typedef string TimeUnit (ts.enum="true")
const TimeUnit TimeUnit_Day = "day"
const TimeUnit TimeUnit_Week = "week"

typedef string TaskType (ts.enum="true")
const TaskType TaskType_AutoEval = "auto_evaluate" // 自动评测

typedef string TaskStatus (ts.enum="true")
const TaskStatus TaskStatus_Unstarted = "unstarted"   // 未启动
const TaskStatus TaskStatus_Running = "running"       // 正在运行
const TaskStatus TaskStatus_Failed = "failed"         // 失败
const TaskStatus TaskStatus_Success = "success"       // 成功
const TaskStatus TaskStatus_Pending = "pending"       // 中止
const TaskStatus TaskStatus_Disabled = "disabled"     // 禁用

// Task
struct Task {
    1: optional i64 id                                             // 任务 id
    2: required string name                                        // 名称
    3: optional string description                                 // 描述
    4: optional i64 workspace_id                                   // 所在空间
    5: required TaskType task_type                                 // 类型
    6: optional TaskStatus task_status                             // 状态
    7: optional Rule rule                                          // 规则
    8: optional TaskConfig task_config                             // 配置
    9: optional TaskDetail task_detail                             // 任务状态详情

    100: optional common.BaseInfo base_info                        // 基础信息
}

// Rule
struct Rule {
    1: optional filter.FilterFields  span_filters // Span 过滤条件
    2: optional Sampler sampler                   // 采样配置
    3: optional EffectiveTime effective_time      // 生效时间窗口
}

struct Sampler {
    1: optional double sample_rate                     // 采样率
    2: optional i64 sample_size                        // 采样上限
    3: optional bool is_cycle                          // 是否启动任务循环
    4: optional i64 cycle_count                        // 采样单次上限
    5: optional i64 cycle_interval                     // 循环间隔
    6: optional TimeUnit cycle_time_unit               // 循环时间单位
}

struct EffectiveTime {
    1: optional i64 start_at       // ms timestamp
    2: optional i64 end_at         // ms timestamp
}


// TaskConfig
struct TaskConfig {
    1: optional list<AutoEvaluateConfig> auto_evaluate_configs               // 配置的评测规则信息
}

struct AutoEvaluateConfig {
    1: required i64 evaluator_version_id
    2: required i64 evaluator_id
    3: required list<FieldMapping> field_mappings
}

// TaskDetail
struct TaskDetail {
    1: optional i64 success_count
    2: optional i64 failed_count
}

struct FieldMapping {
    1: required export_dataset.FieldSchema field_schema   // 数据集字段约束
    2: required string trace_field_key
    3: required string trace_field_jsonpath
    4: optional string eval_set_name
}

// TaskRun
struct TaskRun {
    1: optional i64 id                                             // 任务 run id
    2: optional i64 workspace_id                                   // 所在空间
    3: optional i64 task_id                                        // 任务 id
    4: optional TaskType task_type                                 // 类型
    5: required i64 start_run_at
    6: required i64 end_run_at
    7: optional TaskRunConfig task_run_config                      // 配置
}
struct TaskRunConfig {
    1: optional AutoEvaluateRunConfig auto_evaluate_run_config               // 自动评测对应的运行配置信息
}
struct AutoEvaluateRunConfig {
    1: required i64 evaluator_version_id
    2: required i64 evaluator_id
    3: required list<FieldMapping> field_mappings
}