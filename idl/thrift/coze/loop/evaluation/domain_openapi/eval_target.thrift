namespace go coze.loop.evaluation.domain_openapi.eval_target
include "common.thrift"

typedef string EvalTargetType(ts.enum="true")
const EvalTargetType EvalTargetType_CozeBot = "coze_bot"
const EvalTargetType EvalTargetType_CozeLoopPrompt = "coze_loop_prompt"
const EvalTargetType EvalTargetType_Trace = "trace"
const EvalTargetType EvalTargetType_CozeWorkflow = "coze_workflow"
const EvalTargetType EvalTargetType_VolcengineAgent = "volcengine_agent"
const EvalTargetType EvalTargetType_CustomRPCServer = "custom_rpc_server"

typedef string CozeBotInfoType(ts.enum="true")
const CozeBotInfoType CozeBotInfoType_DraftBot = "draft_bot"
const CozeBotInfoType CozeBotInfoType_ProductBot = "product_bot"

typedef string SubmitStatus(ts.enum="true")
const SubmitStatus SubmitStatus_UnSubmit = "unSubmit"
const SubmitStatus SubmitStatus_Submitted = "submitted"

typedef string EvalTargetRunStatus(ts.enum="true")
const EvalTargetRunStatus EvalTargetRunStatus_Success = "success"
const EvalTargetRunStatus EvalTargetRunStatus_Fail = "fail"

typedef string Region (ts.enum="true")
const Region Region_BOE = "boe"
const Region Region_CN = "cn"
const Region Region_I18N = "i18n"

typedef string AccessProtocol (ts.enum="true")
const AccessProtocol AccessProtocol_RPC = "rpc"
const AccessProtocol AccessProtocol_RPCOld = "rpc_old"
const AccessProtocol AccessProtocol_FaasHTTP = "faas_http"
const AccessProtocol AccessProtocol_FaasHTTPOld = "faas_http_old"

typedef string HTTPMethod (ts.enum="true")
const HTTPMethod HTTPMethod_Get = "get"
const HTTPMethod HTTPMethod_Post = "post"


struct CustomEvalTarget {
    1: optional string id // 唯一键，平台不消费，仅做透传
    2: optional string name    // 名称，平台用于展示在对象搜索下拉列表
    3: optional string avatar_url    // 头像url，平台用于展示在对象搜索下拉列表

    10: optional map<string, string> ext    // 扩展字段，目前主要存储旧版协议response中的额外字段：object_type(旧版ID)、object_meta、space_id
}

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
    3: optional string runtime_param_json_demo

    // 101-200 EvalTarget类型
    // EvalTargetType=1 时，传参此字段。 评测对象为 EvalPrompt 时, 需要设置 Prompt 信息
    102: optional EvalPrompt prompt
    // EvalTargetType=6 时，传参此字段。 评测对象为 CustomRPCServer 时, 需要设置 CustomRPCServer 信息
    105: optional CustomRPCServer custom_rpc_server
}

struct EvalTargetRecord  {
    // 基础信息
    1: optional i64 id (api.js_conv='true', go.tag='json:"id"')// 评估记录ID
    2: optional i64 target_id (api.js_conv='true', go.tag='json:"target_id"')
    3: optional i64 target_version_id (api.js_conv='true', go.tag='json:"target_version_id"')
    4: optional i64 item_id (api.js_conv='true', go.tag='json:"item_id"') // 评测集数据项ID
    5: optional i64 turn_id (api.js_conv='true', go.tag='json:"turn_id"') // 评测集数据项轮次ID

    // 运行数据
    20: optional EvalTargetOutputData eval_target_output_data  // 输出数据
    21: optional EvalTargetRunStatus status

    // 系统信息
    50: optional string logid
    51: optional string trace_id

    100: optional common.BaseInfo base_info
}

struct EvalTargetOutputData {
    1: optional map<string, common.Content> output_fields           // 输出字段，目前key只支持actual_output
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

struct EvalPrompt{
    1: optional i64 prompt_id (api.js_conv='str', go.tag='json:"prompt_id"')
    2: optional string version
    3: optional string name  // DTO使用，不存数据库
    4: optional string prompt_key  // DTO使用，不存数据库
    5: optional SubmitStatus submit_status  // DTO使用，不存数据库
    6: optional string description  // DTO使用，不存数据库
}

struct CustomRPCServer {
    1: optional i64 id  (api.js_conv='str', go.tag='json:"id"')  // 应用ID

    2: optional string name    // DTO使用，不存数据库
    3: optional string description // DTO使用，不存数据库

    // 注意以下信息会存储到DB，也就是说实验创建时以下内容就确定了，运行时直接从评测DB中获取，而不是实时从app模块拉
    10: optional string server_name
    11: optional AccessProtocol access_protocol  // 接入协议
    12: optional list<Region> regions
    13: optional string cluster
    14: optional HTTPInfo invoke_http_info // 执行http信息
    15: optional HTTPInfo async_invoke_http_info // 异步执行http信息，如果用户选了异步就传入这个字段
    16: optional bool need_search_target // 是否需要搜索对象
    17: optional HTTPInfo search_http_info  // 搜索对象http信息
    18: optional CustomEvalTarget custom_eval_target   // 搜索对象返回的信息
    19: optional bool is_async    // 是否异步


    20: optional Region exec_region // 执行区域
    21: optional string exec_env // 执行环境
    22: optional i64 timeout // 执行超时时间，单位ms
    23: optional i64 async_timeout // 异步执行超时时间，单位ms

    50: optional map<string, string> ext
}

struct HTTPInfo {
    1: optional HTTPMethod method
    2: optional string path
}
