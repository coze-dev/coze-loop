namespace go coze.loop.prompt.openapi

include "../../../base.thrift"
include "./domain_openapi/prompt.thrift"
include "../extra.thrift"

service PromptOpenAPIService {
    BatchGetPromptByPromptKeyResponse BatchGetPromptByPromptKey(1: BatchGetPromptByPromptKeyRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/mget')
    ExecuteResponse Execute(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute")
    ExecuteStreamingResponse ExecuteStreaming(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute_streaming", streaming.mode='server')
    ListPromptBasicResponse ListPromptBasic(1: ListPromptBasicRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/list')
}

struct BatchGetPromptByPromptKeyRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional list<prompt.PromptQuery> queries (api.body="queries")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct BatchGetPromptByPromptKeyResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.PromptResultData data

    255: optional base.BaseResp BaseResp
}

struct ExecuteRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"') // 工作空间ID
    2: optional prompt.PromptQuery prompt_identifier (api.body="prompt_identifier") // Prompt 标识

    10: optional list<prompt.VariableVal> variable_vals (api.body="variable_vals") // 变量值
    11: optional list<prompt.Message> messages (api.body="messages") // 消息

    20: optional list<prompt.Tool> custom_tools (api.body="custom_tools") // 自定义工具
    21: optional prompt.ToolCallConfig custom_tool_call_config (api.body="custom_tool_call_config") // 自定义工具调用配置
    22: optional prompt.ModelConfig custom_model_config (api.body="custom_model_config") // 自定义模型配置
    23: optional prompt.ResponseAPIConfig response_api_config (api.body="response_api_config") // response api 配置
    24: optional prompt.AccountMode account_mode (api.body="account_mode") // 账号模式（兼容字段）
    26: optional prompt.UsageScenario usage_scenario (api.body="usage_scenario") // 使用场景（兼容字段）
    28: optional string release_label (api.body="release_label") // 发布标签（兼容字段）
    29: optional prompt.ToolCallConfig custom_tool_config (api.body="custom_tool_config") // 自定义工具配置（兼容字段）

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct ExecuteResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.ExecuteData data

    255: optional base.BaseResp BaseResp
}

struct ExecuteStreamingResponse {
    1: optional string id
    2: optional string event
    3: optional i64 retry
    4: optional prompt.ExecuteStreamingData data

    255: optional base.BaseResp BaseResp
}

struct ListPromptBasicRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    2: optional i32 page_number (api.body="page_number", vt.gt = "0")
    3: optional i32 page_size (api.body="page_size", vt.gt = "0", vt.le = "200")
    4: optional string key_word (api.body="key_word") // name/key前缀匹配
    5: optional string creator (api.body="creator") // 创建人
    6: optional map<string, string> extra (api.body="extra") // 额外查询条件

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct ListPromptBasicResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.ListPromptBasicData data

    255: optional base.BaseResp BaseResp
}
