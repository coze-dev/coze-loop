namespace go coze.loop.observability.domain.task

include "common.thrift"
include "filter.thrift"
include "export_dataset.thrift"

typedef string TimeUnit (ts.enum="true")
const TimeUnit TimeUnit_Day = "day"
const TimeUnit TimeUnit_Week = "week"
const TimeUnit TimeUnit_Null = "null"

typedef string TaskType (ts.enum="true")
const TaskType TaskType_AutoEval = "auto_evaluate"          // 自动评测
const TaskType TaskType_AutoDataReflow = "auto_data_reflow" // 数据回流

typedef string TaskRunType (ts.enum="true")
const TaskRunType TaskRunType_BackFill = "back_fill"     // 历史数据回填
const TaskRunType TaskRunType_NewData = "new_data"       // 新数据

typedef string TaskStatus (ts.enum="true")
const TaskStatus TaskStatus_Unstarted = "unstarted"   // 未启动
const TaskStatus TaskStatus_Running = "running"       // 正在运行
const TaskStatus TaskStatus_Failed = "failed"         // 失败
const TaskStatus TaskStatus_Success = "success"       // 成功
const TaskStatus TaskStatus_Pending = "pending"       // 中止
const TaskStatus TaskStatus_Disabled = "disabled"     // 禁用

typedef string RunStatus (ts.enum="true")
const RunStatus RunStatus_Running = "running"       // 正在运行
const RunStatus RunStatus_Done = "done"           // 完成运行

// Task
struct Task {
    1: optional i64 id  (api.js_conv="true", go.tag='json:"id"')                            // 任务 id
    2: required string name                                                                 // 名称
    3: optional string description                                                          // 描述
    4: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')         // 所在空间
    5: required TaskType task_type                                                          // 类型
    6: optional TaskStatus task_status                                                      // 状态
    7: optional Rule rule                                                                   // 规则
    8: optional TaskConfig task_config                                                      // 配置
    9: optional RunDetail task_detail                                                       // 任务状态详情
    10: optional RunDetail backfill_task_detail                                             // 任务历史数据执行详情

    100: optional common.BaseInfo base_info                                                 // 基础信息
}

// Rule
struct Rule {
    1: optional filter.SpanFilterFields  span_filters       // Span 过滤条件
    2: optional Sampler sampler                             // 采样配置
    3: optional EffectiveTime effective_time                // 生效时间窗口
    4: optional EffectiveTime backfill_effective_time       // 历史数据生效时间窗口
}

struct Sampler {
    1: optional double sample_rate                                                          // 采样率
    2: optional i64 sample_size                                                             // 采样上限
    3: optional bool is_cycle                                                               // 是否启动任务循环
    4: optional i64 cycle_count                                                             // 采样单次上限
    5: optional i64 cycle_interval                                                          // 循环间隔
    6: optional TimeUnit cycle_time_unit                                                    // 循环时间单位
}

struct EffectiveTime {
    1: optional i64 start_at (api.js_conv="true", go.tag='json:"start_at"')      // ms timestamp
    2: optional i64 end_at (api.js_conv="true", go.tag='json:"end_at"')          // ms timestamp
}


// TaskConfig
struct TaskConfig {
    1: optional list<AutoEvaluateConfig> auto_evaluate_configs               // 配置的评测规则信息
    2: optional list<DataReflowConfig> data_reflow_config                    // 配置的数据回流的数据集信息
}

struct DataReflowConfig {
    1: optional i64    dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')   // 数据集id，新增数据集时可为空
    2: optional string dataset_name                                                  // 数据集名称
    3: optional export_dataset.DatasetSchema dataset_schema (vt.not_nil="true")      // 数据集列数据schema
    4: optional list<export_dataset.FieldMapping> field_mappings (vt.min_size="1", vt.max_size="100")
}

struct AutoEvaluateConfig {
    1: required i64 evaluator_version_id (api.js_conv="true", go.tag='json:"evaluator_version_id"')
    2: required i64 evaluator_id (api.js_conv="true", go.tag='json:"evaluator_id"')
    3: required list<EvaluateFieldMapping> field_mappings
}

// RunDetail
struct RunDetail {
    1: optional i64 success_count
    2: optional i64 failed_count
    3: optional i64 total_count
}

struct BackfillDetail {
    1: optional i64 success_count
    2: optional i64 failed_count
    3: optional i64 total_count
    4: optional RunStatus backfill_status
    5: optional string last_span_page_token
}

struct EvaluateFieldMapping {
    1: required export_dataset.FieldSchema field_schema   // 数据集字段约束
    2: required string trace_field_key
    3: required string trace_field_jsonpath
    4: optional string eval_set_name
}

// TaskRun
struct TaskRun {
    1: required i64 id (api.js_conv="true", go.tag='json:"id"')                                 // 任务 run id
    2: required i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')             // 所在空间
    3: required i64 task_id (api.js_conv="true", go.tag='json:"task_id"')                       // 任务 id
    4: required TaskRunType task_type                                                              // 类型
    5: required RunStatus run_status                                                            // 状态
    6: optional RunDetail run_detail                                                            // 任务状态详情
    7: optional BackfillDetail backfill_run_detail                                              // 任务历史数据执行详情
    8: required i64 run_start_at (api.js_conv="true", go.tag='json:"run_start_at"')
    9: required i64 run_end_at (api.js_conv="true", go.tag='json:"run_end_at"')
    10: optional TaskRunConfig task_run_config                                                  // 配置

    100: optional common.BaseInfo base_info                                                     // 基础信息
}
struct TaskRunConfig {
    1: optional AutoEvaluateRunConfig auto_evaluate_run_config               // 自动评测对应的运行配置信息
    2: optional DataReflowRunConfig data_reflow_run_config                         // 数据回流对应的运行配置信息
}
struct AutoEvaluateRunConfig {
    1: required i64 expt_id (api.js_conv="true", go.tag='json:"expt_id"')
    2: required i64 expt_run_id (api.js_conv="true", go.tag='json:"expt_run_id"')
    3: required i64 eval_id (api.js_conv="true", go.tag='json:"eval_id"')
    4: required i64 schema_id (api.js_conv="true", go.tag='json:"schema_id"')
    5: optional string schema
    6: required i64 end_at (api.js_conv="true", go.tag='json:"end_at"')
    7: required i64 cycle_start_at (api.js_conv="true", go.tag='json:"cycle_start_at"')
    8: required i64 cycle_end_at (api.js_conv="true", go.tag='json:"cycle_end_at"')
    9: required string status
}
struct DataReflowRunConfig {
    1: required i64 dataset_id (api.js_conv="true", go.tag='json:"dataset_id"')
    2: required i64 dataset_run_id (api.js_conv="true", go.tag='json:"dataset_run_id"')
    3: required i64 end_at (api.js_conv="true", go.tag='json:"end_at"')
    4: required i64 cycle_start_at (api.js_conv="true", go.tag='json:"cycle_start_at"')
    5: required i64 cycle_end_at (api.js_conv="true", go.tag='json:"cycle_end_at"')
    6: required string status
}