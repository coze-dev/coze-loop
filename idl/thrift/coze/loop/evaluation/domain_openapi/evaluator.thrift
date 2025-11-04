namespace go coze.loop.evaluation.domain_openapi.evaluator

include "common.thrift"

// 评估器类型
typedef string EvaluatorType(ts.enum="true")
const EvaluatorType EvaluatorType_Prompt = "prompt"
const EvaluatorType EvaluatorType_Code = "code"

// 语言类型
typedef string LanguageType(ts.enum="true")
const LanguageType LanguageType_Python = "python"
const LanguageType LanguageType_JS = "javascript"

// 运行状态
typedef string EvaluatorRunStatus(ts.enum="true")
const EvaluatorRunStatus EvaluatorRunStatus_Success = "success"
const EvaluatorRunStatus EvaluatorRunStatus_Failed = "failed"
const EvaluatorRunStatus EvaluatorRunStatus_Processing = "processing"


// Prompt评估器
struct PromptEvaluator {
    1: optional list<common.Message> messages
    2: optional common.ModelConfig model_config
}

// 代码评估器
struct CodeEvaluator {
    1: optional LanguageType language_type
    2: optional string code_content
}

// 评估器内容
struct EvaluatorContent {
    1: optional bool is_receive_chat_history
    2: optional list<common.ArgsSchema> input_schemas

    // 101-200 Evaluator类型
    101: optional PromptEvaluator prompt_evaluator
    102: optional CodeEvaluator code_evaluator
}

// 评估器版本
struct EvaluatorVersion {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')  // 版本ID
    2: optional string version
    3: optional string description

    20: optional EvaluatorContent evaluator_content

    100: optional common.BaseInfo base_info
}

// 评估器
struct Evaluator {
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional string name
    3: optional string description
    4: optional EvaluatorType evaluator_type
    5: optional bool is_draft_submitted
    6: optional string latest_version

    20: optional EvaluatorVersion current_version

    100: optional common.BaseInfo base_info
}

// 评估器结果
struct EvaluatorResult {
    1: optional double score
    2: optional string reasoning
}

// 评估器使用量
struct EvaluatorUsage {
    1: optional i64 input_tokens (api.js_conv = 'true', go.tag = 'json:"input_tokens"')
    2: optional i64 output_tokens (api.js_conv = 'true', go.tag = 'json:"output_tokens"')
}

// 评估器运行错误
struct EvaluatorRunError {
    1: optional i32 code
    2: optional string message
}

// 评估器输出数据
struct EvaluatorOutputData {
    1: optional EvaluatorResult evaluator_result
    2: optional EvaluatorUsage evaluator_usage
    3: optional EvaluatorRunError evaluator_run_error
    4: optional i64 time_consuming_ms (api.js_conv = 'true', go.tag = 'json:"time_consuming_ms"')
}

// 评估器输入数据
struct EvaluatorInputData {
    1: optional list<common.Message> history_messages
    2: optional map<string, common.Content> input_fields
}

// 评估器执行记录
struct EvaluatorRecord {
    // 基础信息
    1: optional i64 id (api.js_conv = 'true', go.tag = 'json:"id"')
    2: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    3: optional i64 item_id (api.js_conv = 'true', go.tag = 'json:"item_id"')
    4: optional i64 turn_id (api.js_conv = 'true', go.tag = 'json:"turn_id"')

    // 运行数据
    20: optional EvaluatorRunStatus status
    21: optional EvaluatorOutputData evaluator_output_data

    // 系统信息
    50: optional string logid
    51: optional string trace_id

    100: optional common.BaseInfo base_info
}