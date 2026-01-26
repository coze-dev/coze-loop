namespace go volcengine.agentkit.llm.commercial

include "../../../base.thrift"
include "../../../coze/loop/llm/domain/manage.thrift"
include "../../../coze/loop/llm/domain/common.thrift"

typedef string Scenario (ts.enum="true")
const Scenario scenario_default = "default"
const Scenario scenario_prompt_debug = "prompt_debug"
const Scenario scenario_prompt_as_a_service = "prompt_as_a_service"
const Scenario scenario_eval_target = "eval_target"
const Scenario scenario_evaluator = "evaluator"
const Scenario scenario_data_synthesis = "data_synthesis"

struct ListModelsRequest {
    1: optional i64 workspace_id (api.js_conv = 'true', vt.not_nil = 'true', vt.gt = '0', go.tag = 'json:"workspace_id"', api.query = 'WorkspaceId')
    2: optional Scenario scenario
    100: optional string cookie (api.header = 'cookie')
    127: optional i32 page_size
    128: optional string page_token
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

struct GetModelRequest {
    1: optional i64 workspace_id (api.js_conv = 'true', vt.not_nil = 'true', vt.gt = '0', api.query = 'WorkspaceId')
    2: optional i64 model_id (api.js_conv = 'true', api.query = 'ModelId')
    3: optional string identification
    4: optional manage.Protocol protocol
    100: optional string cookie (api.header = 'cookie')
    250: optional string project_name (api.query = 'ProjectName')
    255: optional base.Base Base
}

struct ListModelsResponse {
    1: optional list<manage.Model> models
    127: optional bool has_more
    128: optional string next_page_token
    129: optional i32 total

    255: base.BaseResp BaseResp
}

struct GetModelResponse {
    1: optional manage.Model model

    255: base.BaseResp BaseResp
}

service LLMCommercialService {

    ListModelsResponse ListModels(1: ListModelsRequest request) (
        api.post = '/api/llm/v1/ListModels', api.category = 'loopmodel', api.tag = 'volc-agentkit-gen', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

    GetModelResponse GetModel(1: GetModelRequest req) (
        api.post = '/api/llm/v1/GetModel', api.category = 'loopmodel', api.tag = 'volc-agentkit-gen,volc-agentkit-patched', api.top_operation_type = 'query', api.top_is_auth = 'true', api.top_timeout = '1000'
    )

}
