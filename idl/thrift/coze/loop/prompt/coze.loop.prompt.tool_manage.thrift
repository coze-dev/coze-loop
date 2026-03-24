namespace go coze.loop.prompt.tool.manage

include "../../../base.thrift"
include "./domain/tool.thrift"
include "./domain/user.thrift"

service ToolManageService {
    CreateToolResponse CreateTool(1: CreateToolRequest request) (api.post = '/api/prompt/v1/tools')
    GetToolDetailResponse GetToolDetail(1: GetToolDetailRequest request) (api.get = '/api/prompt/v1/tools/:tool_id')
    ListToolResponse ListTool(1: ListToolRequest request) (api.post = '/api/prompt/v1/tools/list')
    SaveToolDetailResponse SaveToolDetail(1: SaveToolDetailRequest request) (api.post = '/api/prompt/v1/tools/:tool_id/drafts/save')
    CommitToolDraftResponse CommitToolDraft(1: CommitToolDraftRequest request) (api.post = '/api/prompt/v1/tools/:tool_id/drafts/commit')
    ListToolCommitResponse ListToolCommit(1: ListToolCommitRequest request) (api.post = '/api/prompt/v1/tools/:tool_id/commits/list')
    BatchGetToolsResponse BatchGetTools(1: BatchGetToolsRequest request) (api.post = '/api/prompt/v1/tools/mget')
}

struct CreateToolRequest {
    1: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional string tool_name (vt.not_nil="true", vt.min_size="1")
    12: optional string tool_description

    21: optional tool.ToolDetail draft_detail

    255: optional base.Base Base
}

struct CreateToolResponse {
    1: optional i64 tool_id (api.js_conv="true", go.tag='json:"tool_id"')

    255: optional base.BaseResp BaseResp
}

struct GetToolDetailRequest {
    1: optional i64 tool_id (api.path='tool_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"tool_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional bool with_commit (api.query="with_commit")
    12: optional string commit_version (api.query="commit_version")

    21: optional bool with_draft (api.query="with_draft")

    255: optional base.Base Base
}

struct GetToolDetailResponse {
    1: optional tool.Tool tool

    255: optional base.BaseResp BaseResp
}

struct ListToolRequest {
    1: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional string key_word
    12: optional list<string> created_bys
    13: optional bool committed_only

    127: optional i32 page_num (vt.not_nil="true", vt.gt="0")
    128: optional i32 page_size (vt.not_nil="true", vt.gt="0", vt.le="100")
    129: optional ListToolOrderBy order_by
    130: optional bool asc

    255: optional base.Base Base
}

struct ListToolResponse {
    1: optional list<tool.Tool> tools

    11: optional list<user.UserInfoDetail> users

    127: optional i32 total

    255: optional base.BaseResp BaseResp
}

typedef string ListToolOrderBy (ts.enum="true")
const ListToolOrderBy ListToolOrderBy_CommittedAt = "committed_at"
const ListToolOrderBy ListToolOrderBy_CreatedAt = "created_at"

struct SaveToolDetailRequest {
    1: optional i64 tool_id (api.path='tool_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"tool_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional tool.ToolDetail tool_detail (vt.not_nil = "true")
    12: optional string base_version

    255: optional base.Base Base
}

struct SaveToolDetailResponse {
    255: optional base.BaseResp BaseResp
}

struct CommitToolDraftRequest {
    1: optional i64 tool_id (api.path='tool_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"tool_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional string commit_version (vt.not_nil="true", vt.min_size="1")
    12: optional string commit_description
    13: optional string base_version

    255: optional base.Base Base
}

struct CommitToolDraftResponse {
    255: optional base.BaseResp BaseResp
}

struct ListToolCommitRequest {
    1: optional i64 tool_id (api.path='tool_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"tool_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')
    3: optional bool with_commit_detail (api.query="with_commit_detail")

    127: optional i32 page_size (vt.not_nil="true", vt.gt="0")
    128: optional string page_token
    129: optional bool asc

    255: optional base.Base Base
}

struct ListToolCommitResponse {
    1: optional list<tool.CommitInfo> tool_commit_infos
    2: optional map<string, tool.ToolDetail> tool_commit_detail_mapping

    11: optional list<user.UserInfoDetail> users

    127: optional bool has_more
    128: optional string next_page_token

    255: optional base.BaseResp BaseResp
}

struct ToolQuery {
    1: optional i64 tool_id (api.js_conv='true', go.tag='json:"tool_id"')
    2: optional string version
}

struct ToolResult {
    1: optional ToolQuery query
    2: optional tool.Tool tool
}

struct BatchGetToolsRequest {
    1: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')
    2: optional list<ToolQuery> queries (vt.min_size="1")

    255: optional base.Base Base
}

struct BatchGetToolsResponse {
    1: optional list<ToolResult> items

    255: optional base.BaseResp BaseResp
}
