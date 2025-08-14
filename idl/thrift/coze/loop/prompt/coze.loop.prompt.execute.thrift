namespace go coze.loop.prompt.execute

include "../../../base.thrift"
include "./domain/prompt.thrift"

service PromptExecuteService {
    ExecuteInternalResponse ExecuteInternal(1: ExecuteInternalRequest req)
}

struct ExecuteInternalRequest {
    1: optional i64 prompt_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')
    3: optional string version (vt.not_nil='true', vt.min_size='1')
    4: optional list<prompt.Message> messages
    5: optional list<prompt.VariableVal> variable_vals
    6: optional prompt.OverridePromptParams override_prompt_params

    101: optional prompt.Scenario scenario

    255: optional base.Base Base
}

struct ExecuteInternalResponse {
    1: optional prompt.Message message
    2: optional string finish_reason
    3: optional prompt.TokenUsage usage

    255: optional base.BaseResp BaseResp
}

