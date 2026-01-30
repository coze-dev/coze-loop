namespace go coze.loop.prompt.debug

include "../../../base.thrift"
include "./domain/prompt.thrift"

service PromptDebugService {
    DebugStreamingResponse DebugStreaming(1: DebugStreamingRequest req) (api.category="prompt_debug", api.post='/api/prompt/v1/prompts/:prompt_id/debug_streaming', api.op_type = 'query', api.tag = 'volc-agentkit', streaming.mode='server')
    SaveDebugContextResponse SaveDebugContext(1: SaveDebugContextRequest req) (api.category="prompt_debug", api.post='/api/prompt/v1/prompts/:prompt_id/debug_context/save', api.op_type = 'update', api.tag = 'volc-agentkit')
    GetDebugContextResponse GetDebugContext(1: GetDebugContextRequest req) (api.category="prompt_debug", api.get='/api/prompt/v1/prompts/:prompt_id/debug_context/get', api.op_type = 'query', api.tag = 'volc-agentkit')
    ListDebugHistoryResponse ListDebugHistory(1: ListDebugHistoryRequest req) (api.category="prompt_debug", api.get='/api/prompt/v1/prompts/:prompt_id/debug_history/list', api.op_type = 'list', api.tag = 'volc-agentkit')
}

struct DebugStreamingRequest {
    1: optional prompt.Prompt prompt (vt.not_nil="true")
    2: optional list<prompt.Message> messages
    3: optional list<prompt.VariableVal> variable_vals
    4: optional list<prompt.MockTool> mock_tools

    101: optional bool single_step_debug (vt.not_nil="true")
    102: optional string debug_trace_key

    255: optional base.Base Base
}

struct DebugStreamingResponse {
    1: optional prompt.Message delta
    2: optional string finish_reason
    3: optional prompt.TokenUsage usage
    4: optional i64 debug_id (api.js_conv='true', go.tag='json:"debug_id"')
    5: optional string debug_trace_key

    255: optional base.BaseResp BaseResp
}

struct SaveDebugContextRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')
    3: optional prompt.DebugContext debug_context (vt.not_nil="true")

    255: optional base.Base Base
}

struct SaveDebugContextResponse {
    255: optional base.BaseResp BaseResp
}

struct GetDebugContextRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.query='workspace_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    255: optional base.Base Base
}

struct GetDebugContextResponse {
    1: optional prompt.DebugContext debug_context

    255: optional base.BaseResp BaseResp
}

struct ListDebugHistoryRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.query='workspace_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')
    3: optional i32 days_limit (api.query='days_limit', vt.not_nil='true', vt.gt='0')
    4: optional i32 page_size (api.query='page_size', vt.not_nil='true', vt.gt='0')
    5: optional string page_token (api.query='page_token')

    255: optional base.Base Base
}

struct ListDebugHistoryResponse {
    1: optional list<prompt.DebugLog> debug_history
    2: optional bool has_more
    3: optional string next_page_token

    255: optional base.BaseResp BaseResp
}
