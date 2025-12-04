namespace go common

struct Trajectory {
    // trace_id
    1: optional string id
    // 根节点，记录整个轨迹的信息
    2: optional RootStep root_step
    // agent step列表，记录轨迹中agent执行信息
    3: optional list<AgentStep> agent_steps
}

struct RootStep {
    1: optional string id       // 唯一ID，trace导入时取span_id
    2: optional string name     // name，trace导入时取span_name
    3: optional string input    // 输入
    4: optional string output   // 输出

    // 系统属性
    100: optional map<string, string> metadata   // 保留字段，可以承载业务自定义的属性
    101: optional BasicInfo basic_info
    102: optional MetricsInfo metrics_info
}

struct AgentStep {
    // 基础属性
    1: optional string id           // 唯一ID，trace导入时取span_id
    2: optional string parent_id    // 父ID， trace导入时取parent_span_id
    3: optional string name         // name，trace导入时取span_name
    4: optional string input        // 输入
    5: optional string output       // 输出

    20: optional list<Step> steps   // 子节点，agent执行内部经历了哪些步骤

    // 系统属性
    100: optional map<string, string> metadata   // 保留字段，可以承载业务自定义的属性
    101: optional BasicInfo basic_info
    102: optional MetricsInfo metrics_info
}

struct Step {
    // 基础属性
    1: optional string id           // 唯一ID，trace导入时取span_id
    2: optional string parent_id    // 父ID， trace导入时取parent_span_id
    3: optional StepType type       // 类型
    4: optional string name         // name，trace导入时取span_name
    5: optional string input        // 输入
    6: optional string output       // 输出

    // 各种类型补充信息
    20: optional ModelInfo model_info // type=model时填充

    // 系统属性
    100: optional map<string, string> metadata   // 保留字段，可以承载业务自定义的属性
    101: optional BasicInfo basic_info
}

typedef string StepType(ts.enum="true")
const StepType StepType_Agent = "agent"
const StepType StepType_Model = "model"
const StepType StepType_Tool = "tool"

struct ModelInfo {
    1: optional i32 input_tokens
    2: optional i32 output_tokens
    3: optional string latency_first_resp // 首包耗时，单位毫秒
    4: optional i32 reasoning_tokens
    5: optional i32 input_read_cached_tokens
    6: optional i32 input_creation_cached_tokens
}

struct BasicInfo {
    1: optional string started_at // 单位毫秒
    2: optional string duration  // 单位毫秒
    3: optional Error error
}

struct Error {
    1: optional i32 code
    2: optional string msg
}

struct MetricsInfo {
    1: optional string llm_duration // 单位毫秒
    2: optional string tool_duration // 单位毫秒
    3: optional map<i32, list<string>> tool_errors // Tool错误分布，格式为：错误码-->list<ToolStepID>
    4: optional double tool_error_rate // Tool错误率
    5: optional map<i32, list<string>> model_errors // Model错误分布，格式为：错误码-->list<ModelStepID>
    6: optional double model_error_rate // Model错误率
    7: optional double tool_step_proportion // Tool Step占比(分母是总子Step)
}