namespace go coze.loop.prompt.manage

include "../../../base.thrift"
include "./domain/prompt.thrift"
include "./domain/user.thrift"


service PromptManageService {

// --------------- Prompt管理 --------------- //

    // 增
    CreatePromptResponse CreatePrompt(1: CreatePromptRequest request) (api.post = '/api/prompt/v1/prompts')
    ClonePromptResponse ClonePrompt(1: ClonePromptRequest request) (api.post = '/api/prompt/v1/prompts/:prompt_id/clone')

    // 删
    DeletePromptResponse DeletePrompt(1: DeletePromptRequest request) (api.delete = '/api/prompt/v1/prompts/:prompt_id')

    // 查
    GetPromptResponse GetPrompt(1: GetPromptRequest request) (api.get = '/api/prompt/v1/prompts/:prompt_id')
    BatchGetPromptResponse BatchGetPrompt(1: BatchGetPromptRequest request)
    ListPromptResponse ListPrompt(1: ListPromptRequest request) (api.post = '/api/prompt/v1/prompts/list')

    // 改
    UpdatePromptResponse UpdatePrompt(1: UpdatePromptRequest request) (api.put = '/api/prompt/v1/prompts/:prompt_id')
    SaveDraftResponse SaveDraft(1: SaveDraftRequest request) (api.post = '/api/prompt/v1/prompts/:prompt_id/drafts/save')

// --------------- Prompt版本管理 --------------- //

    ListCommitResponse ListCommit(1: ListCommitRequest request) (api.post = '/api/prompt/v1/prompts/:prompt_id/commits/list')
    CommitDraftResponse CommitDraft(1: CommitDraftRequest request) (api.post = '/api/prompt/v1/prompts/:prompt_id/drafts/commit')
    RevertDraftFromCommitResponse RevertDraftFromCommit(1: RevertDraftFromCommitRequest request) (api.post = '/api/prompt/v1/prompts/:prompt_id/drafts/revert_from_commit')

}

// --------------- Prompt管理 --------------- //

struct CreatePromptRequest {
    1: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional string prompt_name (vt.not_nil="true", vt.min_size="1")
    12: optional string prompt_key (vt.not_nil="true", vt.min_size="1")
    13: optional string prompt_description

    21: optional prompt.PromptDetail draft_detail

    255: optional base.Base Base
}
struct CreatePromptResponse {
    1: optional i64 prompt_id (api.js_conv="true", go.tag='json:"prompt_id"')

    255: optional base.BaseResp  BaseResp
}

struct ClonePromptRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional string commit_version (vt.not_nil="true", vt.min_size="1")

    11: optional string cloned_prompt_name (vt.not_nil="true", vt.min_size="1")
    12: optional string cloned_prompt_key (vt.not_nil="true", vt.min_size="1")
    13: optional string cloned_prompt_description

    255: optional base.Base Base
}
struct ClonePromptResponse {
    1: optional i64 cloned_prompt_id (api.js_conv="true", go.tag='json:"cloned_prompt_id"')

    255: optional base.BaseResp  BaseResp
}

struct DeletePromptRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')

    255: optional base.Base Base
}
struct DeletePromptResponse {
    255: optional base.BaseResp  BaseResp
}

struct GetPromptRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional i64 workspace_id (api.query="workspace_id", api.js_conv='true', go.tag='json:"workspace_id"')

    11: optional bool with_commit (api.query="with_commit")
    12: optional string commit_version (api.query="commit_version")

    21: optional bool with_draft (api.query="with_draft")

    31: optional bool with_default_config (api.query="with_default_config")

    255: optional base.Base Base
}
struct GetPromptResponse {
    1: optional prompt.Prompt prompt

    11: optional prompt.PromptDetail default_config

    255: optional base.BaseResp  BaseResp
}

struct PromptQuery {
    1: optional i64 prompt_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')

    11: optional bool with_commit
    12: optional string commit_version
}

struct BatchGetPromptRequest {
    1: optional list<PromptQuery> queries (vt.min_size = "1")

    255: optional base.Base Base
}

struct BatchGetPromptResponse {
    1: optional list<PromptResult> results

    255: optional base.BaseResp  BaseResp
}

struct PromptResult {
    1: optional PromptQuery query
    2: optional prompt.Prompt prompt
}

struct ListPromptRequest {
    1: optional i64 workspace_id (api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"workspace_id"')

    11: optional string key_word
    12: optional list<string> created_bys
    13: optional bool committed_only

    127: optional i32 page_num (vt.not_nil="true", vt.gt="0")
    128: optional i32 page_size (vt.not_nil="true", vt.gt="0", vt.le="100")
    129: optional ListPromptOrderBy order_by
    130: optional bool asc

    255: optional base.Base Base
}
struct ListPromptResponse {
    1: optional list<prompt.Prompt> prompts

    11: optional list<user.UserInfoDetail> users

    127: optional i32 total

    255: optional base.BaseResp BaseResp
}
typedef string ListPromptOrderBy (ts.enum="true")
const ListPromptOrderBy ListPromptOrderBy_CommitedAt = "committed_at"
const ListPromptOrderBy ListPromptOrderBy_CreatedAt = "created_at"

struct UpdatePromptRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', go.tag='json:"prompt_id"')

    11: optional string prompt_name (vt.not_nil="true", vt.min_size="1")
    12: optional string prompt_description

    255: optional base.Base Base
}
struct UpdatePromptResponse {
    255: optional base.BaseResp  BaseResp
}

struct SaveDraftRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')

    11: optional prompt.PromptDraft prompt_draft (vt.not_nil = "true")

    255: optional base.Base Base
}
struct SaveDraftResponse {
    1: optional prompt.DraftInfo draft_info

    255: optional base.BaseResp  BaseResp
}

struct CommitDraftRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')

    11: optional string commit_version (vt.not_nil="true", vt.min_size="1")
    12: optional string commit_description

    255: optional base.Base Base
}
struct CommitDraftResponse {
    255: optional base.BaseResp  BaseResp
}

// 搜索Prompt提交版本
struct ListCommitRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')

    127: optional i32 page_size (vt.not_nil="true", vt.gt="0")
    128: optional string page_token
    129: optional bool asc

    255: optional base.Base Base
}
struct ListCommitResponse {
    1: optional list<prompt.CommitInfo> prompt_commit_infos

    11: optional list<user.UserInfoDetail> users

    127: optional bool has_more
    128: optional string next_page_token

    255: optional base.BaseResp  BaseResp
}

struct RevertDraftFromCommitRequest {
    1: optional i64 prompt_id (api.path='prompt_id', api.js_conv='true', vt.not_nil='true', vt.gt='0', go.tag='json:"prompt_id"')
    2: optional string commit_version_reverting_from (vt.not_nil="true", vt.min_size="1")

    255: optional base.Base Base
}
struct RevertDraftFromCommitResponse {
    255: optional base.BaseResp  BaseResp
}
