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

// 消息角色
typedef string Role(ts.enum="true")
const Role Role_System = "system"
const Role Role_User = "user"
const Role Role_Assistant = "assistant"

// 消息结构
struct Message {
    1: optional Role role
    2: optional common.Content content
    3: optional map<string, string> ext
}

// Prompt评估器
struct PromptEvaluator {
    1: optional list<Message> message_list
    2: optional common.ModelConfig model_config
}

// 代码评估器
struct CodeEvaluator {
    1: optional LanguageType language_type
    2: optional string code
}

// 评估器内容
struct EvaluatorContent {
    1: optional bool receive_chat_history
    2: optional list<common.ArgsSchema> input_schemas
    3: optional PromptEvaluator prompt_evaluator
    4: optional CodeEvaluator code_evaluator
}

// 评估器版本
struct EvaluatorVersion {
    1: optional string evaluator_version_id
    2: optional string version
    3: optional string description
    4: optional EvaluatorContent evaluator_content
    5: optional common.BaseInfo base_info
}

// 评估器
struct Evaluator {
    1: optional string evaluator_id
    2: optional string name
    3: optional string description
    4: optional EvaluatorType evaluator_type
    5: optional bool draft_submitted
    6: optional string latest_version
    7: optional EvaluatorVersion current_version
    8: optional common.BaseInfo base_info
}

// 评估器结果
struct EvaluatorResult {
    1: optional double score
    2: optional string reasoning
}

// 评估器使用量
struct EvaluatorUsage {
    1: optional string input_tokens
    2: optional string output_tokens
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
    4: optional string time_consuming_ms
}

// 评估器输入数据
struct EvaluatorInputData {
    1: optional list<Message> history_messages
    2: optional map<string, common.Content> input_fields
}

// 评估器执行记录
struct EvaluatorRecord {
    1: optional string record_id
    2: optional string evaluator_version_id
    3: optional string trace_id
    4: optional EvaluatorRunStatus status
    5: optional EvaluatorInputData evaluator_input_data
    6: optional EvaluatorOutputData evaluator_output_data
    7: optional common.BaseInfo base_info
    8: optional map<string, string> ext
}