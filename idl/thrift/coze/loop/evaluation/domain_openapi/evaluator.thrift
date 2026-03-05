namespace go coze.loop.evaluation.domain_openapi.evaluator

include "common.thrift"

// 评估器类型
typedef string EvaluatorType(ts.enum="true")
const EvaluatorType EvaluatorType_Prompt = "prompt"
const EvaluatorType EvaluatorType_Code = "code"
const EvaluatorType EvaluatorType_CustomRPC = "custom_rpc"

// 语言类型
typedef string LanguageType(ts.enum="true")
const LanguageType LanguageType_Python = "python"
const LanguageType LanguageType_JS = "javascript"

// 运行状态
typedef string EvaluatorRunStatus(ts.enum="true")
const EvaluatorRunStatus EvaluatorRunStatus_Unknown = "unknown"
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

// 接入协议（仅保留当前版本，不含 old）
typedef string EvaluatorAccessProtocol(ts.enum="true")
const EvaluatorAccessProtocol EvaluatorAccessProtocol_RPC = "rpc"
const EvaluatorAccessProtocol EvaluatorAccessProtocol_FaasHTTP = "faas_http"

// HTTP 方法
typedef string EvaluatorHTTPMethod(ts.enum="true")
const EvaluatorHTTPMethod EvaluatorHTTPMethod_Get = "get"
const EvaluatorHTTPMethod EvaluatorHTTPMethod_Post = "post"

// 自定义评估器 HTTP 调用信息
struct EvaluatorHTTPInfo {
    1: optional EvaluatorHTTPMethod method
    2: optional string path
}

// 自定义评估器 (RPC)，与 domain/evaluator 对齐，EvaluatorAccessProtocol 不含 old 版本
struct CustomRPCEvaluator {
    1: optional string provider_evaluator_code   // 自定义评估器编码
    2: optional EvaluatorAccessProtocol access_protocol  // rpc / faas_http
    3: optional string service_name
    4: optional string cluster
    5: optional EvaluatorHTTPInfo invoke_http_info

    10: optional i64 timeout    // ms
    11: optional common.RateLimit rate_limit
    12: optional map<string, string> ext
}

// 评估器内容
struct EvaluatorContent {
    1: optional bool is_receive_chat_history
    2: optional list<common.ArgsSchema> input_schemas
    3: optional list<common.ArgsSchema> output_schemas

    // 101-200 Evaluator类型
    101: optional PromptEvaluator prompt_evaluator
    102: optional CodeEvaluator code_evaluator
    103: optional CustomRPCEvaluator custom_rpc_evaluator
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
    2: optional i64 workspace_id (api.js_conv = 'true', go.tag = 'json:"workspace_id"')
    3: optional string name
    4: optional string description
    5: optional EvaluatorType evaluator_type
    6: optional bool is_draft_submitted
    7: optional string latest_version

    20: optional EvaluatorVersion current_version

    100: optional common.BaseInfo base_info
}

// 评估器结果
struct EvaluatorResult {
    1: optional double score
    2: optional string reasoning
    3: optional Correction correction
}

struct Correction {
    1: optional double score
    2: optional string explain
    3: optional string updated_by
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
    11: optional string stdout
}

// 评估器输入数据
struct EvaluatorInputData {
    1: optional list<common.Message> history_messages
    2: optional map<string, common.Content> input_fields
    3: optional map<string, common.Content> evaluate_dataset_fields
    4: optional map<string, common.Content> evaluate_target_output_fields
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

struct EvaluatorRunConfig {
    1: optional string env
    2: optional common.RuntimeParam evaluator_runtime_param
}

// 评估器ID版本项
struct EvaluatorIDVersionItem {
    1: optional i64 evaluator_id (api.js_conv = 'true', go.tag = 'json:"evaluator_id"')
    2: optional string version (api.js_conv = 'true', go.tag = 'json:"version"')
    3: optional EvaluatorRunConfig run_config (go.tag = 'json:"run_config"')
    4: optional i64 evaluator_version_id (api.js_conv = 'true', go.tag = 'json:"evaluator_version_id"')
    5: optional double score_weight (go.tag = 'json:"score_weight"')
}

// 筛选器逻辑操作符
typedef string EvaluatorFilterLogicOp(ts.enum="true")
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_Unknown = "unknown"
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_And = "and"
const EvaluatorFilterLogicOp EvaluatorFilterLogicOp_Or = "or"

// 筛选器条件
struct EvaluatorFilterCondition {
    1: optional string tag_key
    2: optional string operator
    3: optional string value
}

// 评估器筛选器
struct EvaluatorFilters {
    1: optional list<EvaluatorFilterCondition> filter_conditions
    2: optional EvaluatorFilterLogicOp logic_op
    3: optional list<EvaluatorFilters> sub_filters
}

// 评估器筛选器选项
struct EvaluatorFilterOption {
    1: optional string search_keyword
    2: optional EvaluatorFilters filters
}
