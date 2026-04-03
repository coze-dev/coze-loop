namespace go coze.loop.prompt.openapi

include "../../../base.thrift"
include "./domain_openapi/prompt.thrift"
include "../extra.thrift"

service PromptOpenAPIService {
    BatchGetPromptByPromptKeyResponse BatchGetPromptByPromptKey(1: BatchGetPromptByPromptKeyRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/mget')
    ExecuteResponse Execute(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute")
    ExecuteStreamingResponse ExecuteStreaming(1: ExecuteRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/execute_streaming", streaming.mode='server')
    ListPromptBasicResponse ListPromptBasic(1: ListPromptBasicRequest req) (api.tag="openapi", api.post='/v1/loop/prompts/list')
    CreatePromptOApiResponse CreatePromptOApi(1: CreatePromptOApiRequest req) (api.tag="openapi", api.post="/v1/loop/prompts")
    DeletePromptOApiResponse DeletePromptOApi(1: DeletePromptOApiRequest req) (api.tag="openapi", api.delete="/v1/loop/prompts/:prompt_id")
    GetPromptOApiResponse GetPromptOApi(1: GetPromptOApiRequest req) (api.tag="openapi", api.get="/v1/loop/prompts/:prompt_id")
    SaveDraftOApiResponse SaveDraftOApi(1: SaveDraftOApiRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/:prompt_id/drafts/save")
    ListCommitOApiResponse ListCommitOApi(1: ListCommitOApiRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/:prompt_id/commits/list")
    CommitDraftOApiResponse CommitDraftOApi(1: CommitDraftOApiRequest req) (api.tag="openapi", api.post="/v1/loop/prompts/:prompt_id/drafts/commit")
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

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct ListPromptBasicResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.ListPromptBasicData data

    255: optional base.BaseResp BaseResp
}

struct CreatePromptOApiRequest {
    1: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional string prompt_name (api.body="prompt_name", vt.not_nil="true", vt.min_size="1")
    12: optional string prompt_key (api.body="prompt_key", vt.not_nil="true", vt.min_size="1")
    13: optional string prompt_description (api.body="prompt_description")
    14: optional prompt.PromptType prompt_type (api.body="prompt_type")
    15: optional prompt.SecurityLevel security_level (api.body="security_level")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct CreatePromptOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional i64 prompt_id (api.js_conv="true", go.tag='json:"prompt_id"')

    255: optional base.BaseResp BaseResp
}

struct DeletePromptOApiRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct DeletePromptOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: optional base.BaseResp BaseResp
}

struct GetPromptOApiRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional bool with_commit (api.query="with_commit")
    12: optional string commit_version (api.query="commit_version")
    21: optional bool with_draft (api.query="with_draft")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct GetPromptOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.PromptManage prompt

    255: optional base.BaseResp BaseResp
}

struct SaveDraftOApiRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional prompt.PromptDraft prompt_draft (api.body="prompt_draft", vt.not_nil="true")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct SaveDraftOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional prompt.DraftInfo draft_info (api.body="draft_info")

    255: optional base.BaseResp BaseResp
}

struct ListCommitOApiRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')
    3: optional bool with_commit_detail (api.query="with_commit_detail")

    127: optional i32 page_size (api.body="page_size", vt.not_nil="true", vt.gt="0")
    128: optional string page_token (api.body="page_token")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct ListCommitOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional list<prompt.CommitInfo> prompt_commit_infos (api.body="prompt_commit_infos")
    4: optional map<string, prompt.PromptDetail> prompt_commit_detail_mapping (api.body="prompt_commit_detail_mapping")

    127: optional bool has_more (api.body="has_more")
    128: optional string next_page_token (api.body="next_page_token")

    255: optional base.BaseResp BaseResp
}

struct CommitDraftOApiRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.body="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional string commit_version (api.body="commit_version", vt.not_nil="true", vt.min_size="1")
    12: optional string commit_description (api.body="commit_description")

    254: optional extra.Extra extra (agw.source="not_body_struct")
    255: optional base.Base Base
}

struct CommitDraftOApiResponse {
    1: optional i32 code
    2: optional string msg

    255: optional base.BaseResp BaseResp
}
