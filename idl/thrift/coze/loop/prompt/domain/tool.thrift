namespace go coze.loop.prompt.domain.tool

const string PublicDraftVersion = "$PublicDraft"

struct Tool {
    1: optional i64 id (api.js_conv="true", go.tag='json:"id"')
    2: optional i64 workspace_id (api.js_conv="true", go.tag='json:"workspace_id"')
    3: optional ToolBasic tool_basic
    4: optional ToolCommit tool_commit

    255: optional map<string, string> ext_infos
}

struct ToolBasic {
    1: optional string name
    2: optional string description
    3: optional string latest_committed_version
    4: optional string created_by
    5: optional string updated_by
    6: optional i64 created_at (api.js_conv="true", go.tag='json:"created_at"')
    7: optional i64 updated_at (api.js_conv="true", go.tag='json:"updated_at"')
}

struct ToolCommit {
    1: optional ToolDetail detail
    2: optional CommitInfo commit_info
}

struct CommitInfo {
    1: optional string version
    2: optional string base_version
    3: optional string description
    4: optional string committed_by
    5: optional i64 committed_at (api.js_conv="true", go.tag='json:"committed_at"')
}

struct ToolDetail {
    1: optional string content

    255: optional map<string, string> ext_infos
}
