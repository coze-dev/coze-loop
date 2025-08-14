namespace go coze.loop.evaluation.domain.eval_target

include "common.thrift"

struct EvalTarget {
    // 基本信息
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')  // 一个对象的唯一标识
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"') // 空间ID
    3: optional string source_target_id  // 源对象ID，例如prompt ID
    4: optional EvalTargetType eval_target_type  // 评测对象类型

    // 版本信息
    10: optional EvalTargetVersion eval_target_version  // 目标版本

    // 系统信息
    100: optional common.BaseInfo base_info (go.tag='json:\"base_info\"')
}

struct EvalTargetVersion {
    // 基本信息
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"') // 版本唯一标识
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"')  // 空间ID
    3: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"') // 对象唯一标识
    4: optional string source_target_version  // 源对象版本，例如prompt是0.0.1，bot是版本号12233等
    5: optional EvalTargetContent eval_target_content  // 目标对象内容

    // 系统信息
    100: optional common.BaseInfo base_info (go.tag='json:\"base_info\"')
}

struct EvalTargetContent {
    1: optional list<common.ArgsSchema> input_schemas (go.tag='json:\"input_schemas\"') // 输入schema
    2: optional list<common.ArgsSchema> output_schemas (go.tag='json:\"output_schemas\"') // 输出schema

    // 101-200 EvalTarget类型
    // EvalTargetType=0 时，传参此字段。 评测对象为 CozeBot 时, 需要设置 CozeBot 信息
    101: optional CozeBot coze_bot
    // EvalTargetType=1 时，传参此字段。 评测对象为 EvalPrompt 时, 需要设置 Prompt 信息
    102: optional EvalPrompt prompt
    // EvalTargetType=4 时，传参此字段。 评测对象为 CozeWorkflow 时, 需要设置 CozeWorkflow 信息
    103: optional CozeWorkflow coze_workflow
}

enum EvalTargetType {
    CozeBot = 1 // CozeBot
    CozeLoopPrompt = 2 // Prompt
    Trace = 3 // Trace
    CozeWorkflow = 4
}

struct CozeWorkflow {
    1: optional string id
    2: optional string version

    3: optional string name    // DTO使用，不存数据库
    4: optional string avatar_url // DTO使用，不存数据库
    5: optional string description // DTO使用，不存数据库

    100: optional common.BaseInfo base_info (go.tag='json:\"base_info\"')
}

struct EvalPrompt{
    1: optional i64 prompt_id (api.js_conv='str', go.tag='json:"prompt_id"')
    2: optional string version
    3: optional string name  // DTO使用，不存数据库
    4: optional string prompt_key  // DTO使用，不存数据库
    5: optional SubmitStatus submit_status  // DTO使用，不存数据库
    6: optional string description  // DTO使用，不存数据库
}

enum SubmitStatus {
    Undefined = 0
    UnSubmit // 未提交
    Submitted // 已提交
}

// Coze2.0Bot
struct CozeBot {
    1: optional i64 bot_id (api.js_conv='true', go.tag='json:"bot_id"')
    2: optional string bot_version
    3: optional CozeBotInfoType bot_info_type

    4: optional ModelInfo model_info
    5: optional string bot_name    // DTO使用，不存数据库
    6: optional string avatar_url // DTO使用，不存数据库
    7: optional string description // DTO使用，不存数据库
    8: optional string publish_version // 如果是发布版本则这个字段不为空

    100: optional common.BaseInfo base_info (go.tag='json:\"base_info\"')
}

enum CozeBotInfoType {
   DraftBot = 1 // 草稿 bot
   ProductBot = 2 // 商店 bot
}

struct ModelInfo {
    1: i64    model_id (api.js_conv='true', go.tag='json:"model_id"')
    2: string model_name
    3: string show_name  // DTO使用，不存数据库
    4: i64    max_tokens (api.js_conv='true', go.tag='json:"max_tokens"') // DTO使用，不存数据库
    5: i64    model_family (api.js_conv='true', go.tag='json:"model_family"') // 模型家族信息
    6: optional ModelPlatform platform // 模型平台
}

enum ModelPlatform {
    Unknown = 0;
    GPTOpenAPI = 1;
    MAAS = 2;
}

struct EvalTargetRecord  {
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')// 评估记录ID
    2: optional i64 workspace_id (api.js_conv='true', go.tag='json:"workspace_id"') // 空间ID
    3: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    4: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    5: optional i64 experiment_run_id (api.js_conv='true', go.tag='json:"experiment_run_id"') // 实验执行ID
    6: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"') // 评测集数据项ID
    7: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 评测集数据项轮次ID
    8: optional string trace_id  // 链路ID
    9: optional string log_id  // 链路ID
    10: optional EvalTargetInputData eval_target_input_data // 输入数据
    11: optional EvalTargetOutputData eval_target_output_data  // 输出数据
    12: optional EvalTargetRunStatus status

    100: optional common.BaseInfo base_info (go.tag='json:\"base_info\"')
}

enum EvalTargetRunStatus {
    Unknown = 0
    Success = 1
    Fail = 2
}

struct EvalTargetInputData {
    1: optional list<common.Message> history_messages      // 历史会话记录
    2: optional map <string, common.Content> input_fields       // 变量
    3: optional map<string, string> ext
}

struct EvalTargetOutputData {
    1: optional map<string, common.Content> output_fields           // 变量
    2: optional EvalTargetUsage eval_target_usage             // 运行消耗
    3: optional EvalTargetRunError eval_target_run_error         // 运行报错
    4: optional i64 time_consuming_ms (api.js_conv='true', go.tag='json:\"time_consuming_ms\"') // 运行耗时
}

struct EvalTargetUsage {
    1: i64 input_tokens (api.js_conv='true', go.tag='json:\"input_tokens\"')
    2: i64 output_tokens (api.js_conv='true', go.tag='json:\"output_tokens\"')
}

struct EvalTargetRunError {
    1: optional i32 code (go.tag='json:\"code\"')
    2: optional string message (go.tag='json:\"message\"')
}