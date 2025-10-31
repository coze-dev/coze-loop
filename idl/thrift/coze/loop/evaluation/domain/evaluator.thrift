namespace go coze.loop.evaluation.domain.evaluator

include "common.thrift"
include "../../llm/domain/runtime.thrift"

enum EvaluatorType {
    Prompt = 1
    Code = 2
    CustomRPC = 3
}

typedef string LanguageType(ts.enum="true")
const LanguageType LanguageType_Python = "Python" // 空间
const LanguageType LanguageType_JS = "JS"

enum PromptSourceType {
    BuiltinTemplate = 1
    LoopPrompt = 2
    Custom = 3
}

enum ToolType {
    Function = 1
    GoogleSearch = 2 // for gemini native tool
}

enum TemplateType {
    Prompt = 1
    Code = 2
}

enum EvaluatorRunStatus { // 运行状态, 异步下状态流转, 同步下只有 Success / Fail
    Unknown = 0
    Success = 1
    Fail = 2
}

typedef string EvaluatorTagLangType(ts.enum="true")
const EvaluatorTagLangType EvaluatorTagLangType_Zh = "zh-CN"
const EvaluatorTagLangType EvaluatorTagLangType_En = "en-US"

// Evaluator筛选字段
typedef string EvaluatorTagKey(ts.enum="true")
const EvaluatorTagKey EvaluatorTagKey_Category = "Category"           // 类型筛选 (LLM/Code)
const EvaluatorTagKey EvaluatorTagKey_TargetType = "TargetType"         // 评估对象 (文本/图片/视频等)
const EvaluatorTagKey EvaluatorTagKey_Objective = "Objective"      // 评估目标 (任务完成/内容质量等)
const EvaluatorTagKey EvaluatorTagKey_BusinessScenario = "BusinessScenario"   // 业务场景 (安全风控/AI Coding等)
const EvaluatorTagKey EvaluatorTagKey_BoxType = "BoxType"            // 黑白盒类型
const EvaluatorTagKey EvaluatorTagKey_Name = "Name"               // 评估器名称

typedef string AccessProtocol
const AccessProtocol AccessProtocol_RPC = "rpc"

struct Tool {
    1: ToolType type (go.tag ='mapstructure:"type"')
    2: optional Function function (go.tag ='mapstructure:"function"')
}

struct Function {
    1: string name (go.tag ='mapstructure:"name"')
    2: optional string description (go.tag ='mapstructure:"description"')
    3: optional string parameters (go.tag ='mapstructure:"parameters"')
}

struct PromptEvaluator {
    1: list<common.Message> message_list (go.tag = 'mapstructure:\"message_list\"')
    2: optional common.ModelConfig model_config (go.tag ='mapstructure:"model_config"')
    3: optional PromptSourceType prompt_source_type (go.tag ='mapstructure:"prompt_source_type"')
    4: optional string prompt_template_key (go.tag ='mapstructure:"prompt_template_key"') // 最新版本中存evaluator_template_id
    5: optional string prompt_template_name (go.tag ='mapstructure:"prompt_template_name"')
    6: optional list<Tool> tools (go.tag ='mapstructure:"tools"')
}

struct CodeEvaluator {
    1: optional LanguageType language_type
    2: optional string code_content
    3: optional string code_template_key // code类型评估器模板中code_template_key + language_type是唯一键；最新版本中存evaluator_template_id
    4: optional string code_template_name
}

struct CustomRPCEvaluator {
    1: optional string provider_evaluator_code     // 自定义评估器编码，例如：EvalBot的给“代码生成-代码正确”赋予CN:480的评估器ID
    2: required AccessProtocol access_protocol    // 本期是RPC，后续还可拓展HTTP
    3: optional string service_name
    4: optional string cluster

    10: optional i64 timeout    // ms
}

struct EvaluatorVersion {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')          // 版本id
    3: optional string version
    4: optional string description
    5: optional common.BaseInfo base_info
    6: optional EvaluatorContent evaluator_content
}

struct EvaluatorContent {
    1: optional bool receive_chat_history (go.tag = 'mapstructure:"receive_chat_history"')
    2: optional list<common.ArgsSchema> input_schemas (go.tag = 'mapstructure:"input_schemas"')
    3: optional list<common.ArgsSchema> output_schemas (go.tag = 'mapstructure:"output_schemas"')

    // 101-200 Evaluator类型
    101: optional PromptEvaluator prompt_evaluator (go.tag ='mapstructure:"prompt_evaluator"')
    102: optional CodeEvaluator code_evaluator
    103: optional CustomRPCEvaluator custom_rpc_evaluator
}

struct Evaluator {
    1: optional i64 evaluator_id (api.js_conv = 'true', go.tag = 'json:"evaluator_id"')
    2: optional i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional EvaluatorType evaluator_type
    4: optional string name
    5: optional string description
    6: optional bool draft_submitted
    7: optional common.BaseInfo base_info
    11: optional EvaluatorVersion current_version
    12: optional string latest_version

    20: optional bool builtin (go.tag = 'json:"builtin"')
    21: optional string benchmark (go.tag = 'json:"benchmark"')
    22: optional string vendor (go.tag = 'json:"vendor"')
    23: optional string builtin_visible_version (go.tag = 'json:"builtin_visible_version"')

    100: optional map<EvaluatorTagLangType, map<EvaluatorTagKey, list<string>>> tags (go.tag = 'json:"tags"')
}

struct EvaluatorTemplate {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional EvaluatorType evaluator_type
    4: optional string name
    5: optional string description
    6: optional i64 popularity (go.tag = 'json:"popularity"') // 热度
    7: optional string benchmark (go.tag = 'json:"benchmark"')
    8: optional string vendor (go.tag = 'json:"vendor"')
    9: optional map<EvaluatorTagLangType, map<EvaluatorTagKey, list<string>>> tags (go.tag = 'json:"tags"')

    101: optional EvaluatorContent evaluator_content
    255: optional common.BaseInfo base_info

}

// Evaluator筛选器选项
struct EvaluatorFilterOption {
    1: optional string search_keyword // 模糊搜索关键词，在所有tag中搜索
    2: optional EvaluatorFilters filters  // 筛选条件
}

// Evaluator筛选条件
struct EvaluatorFilters {
    1: optional list<EvaluatorFilterCondition> filter_conditions  // 筛选条件列表
    2: optional EvaluatorFilterLogicOp logic_op  // 逻辑操作符
}

// 筛选逻辑操作符
typedef string EvaluatorFilterLogicOp(ts.enum="true")
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_Unknown = "Unknown"
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_And = "And"    // 与操作
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_Or = "Or"      // 或操作

// Evaluator筛选条件
struct EvaluatorFilterCondition {
    1: required EvaluatorTagKey tag_key  // 筛选字段
    2: required EvaluatorFilterOperatorType operator  // 操作符
    3: required string value  // 操作值
}

// Evaluator筛选操作符
typedef string EvaluatorFilterOperatorType(ts.enum="true")
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_Unknown = "Unknown"
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_Equal = "Equal"        // 等于
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_NotEqual = "NotEqual"     // 不等于
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_In = "In"           // 包含于
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_NotIn = "NotIn"        // 不包含于
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_Like = "Like"         // 模糊匹配
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_IsNull = "IsNull"       // 为空
const EvaluatorFilterOperatorType EvaluatorFilterOperatorType_IsNotNull = "IsNotNull"    // 非空

struct Correction {
    1: optional double score
    2: optional string explain
    3: optional string updated_by
}

struct EvaluatorRecord  {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional i64 experiment_id (api.js_conv = 'true', go.tag = 'json:"experiment_id"')
    3: optional i64 experiment_run_id (api.js_conv = 'true', go.tag = 'json:"experiment_run_id"')
    4: optional i64 item_id (api.js_conv = 'true', go.tag = 'json:"item_id"')
    5: optional i64 turn_id (api.js_conv = 'true', go.tag = 'json:"turn_id"')
    6: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    7: optional string trace_id
    8: optional string log_id
    9: optional EvaluatorInputData evaluator_input_data
    10: optional EvaluatorOutputData evaluator_output_data
    11: optional EvaluatorRunStatus status
    12: optional common.BaseInfo base_info

    20: optional map<string, string> ext
}

struct EvaluatorOutputData {
    1: optional EvaluatorResult evaluator_result
    2: optional EvaluatorUsage evaluator_usage
    3: optional EvaluatorRunError evaluator_run_error
    4: optional i64 time_consuming_ms (api.js_conv = 'true', go.tag = 'json:"time_consuming_ms"')
    11: optional string stdout
}

struct EvaluatorResult {
    1: optional double score
    2: optional Correction correction
    3: optional string reasoning
}

struct EvaluatorUsage {
    1: optional i64 input_tokens (api.js_conv = 'true', go.tag = 'json:"input_tokens"')
    2: optional i64 output_tokens (api.js_conv = 'true', go.tag = 'json:"output_tokens"')
}

struct EvaluatorRunError {
    1: optional i32 code
    2: optional string message
}

struct EvaluatorInputData {
    1: optional list<common.Message> history_messages
    2: optional map<string, common.Content> input_fields
    3: optional map<string, common.Content> evaluate_dataset_fields
    4: optional map<string, common.Content> evaluate_target_output_fields

    100: optional map<string, string> ext
}
