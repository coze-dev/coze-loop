namespace go coze.loop.evaluation.domain.evaluator

include "common.thrift"
include "../../llm/domain/runtime.thrift"

enum EvaluatorType {
    Prompt = 1
    Code = 2
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
    4: optional string prompt_template_key (go.tag ='mapstructure:"prompt_template_key"')
    5: optional string prompt_template_name (go.tag ='mapstructure:"prompt_template_name"')
    6: optional list<Tool> tools (go.tag ='mapstructure:"tools"')
}

struct CodeEvaluator {
    1: optional LanguageType language_type
    2: optional string code_content
    3: optional string code_template_key
    4: optional string code_template_name
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

    // 101-200 Evaluator类型
    101: optional PromptEvaluator prompt_evaluator (go.tag ='mapstructure:"prompt_evaluator"')
    102: optional CodeEvaluator code_evaluator
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
}

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
    3: optional map<string, common.Content> from_eval_set_fields
    4: optional map<string, common.Content> from_eval_target_fields

    100: optional map<string, string> ext
}