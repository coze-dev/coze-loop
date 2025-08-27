namespace go coze.loop.observability.domain.common

typedef string PlatformType (ts.enum="true")
const PlatformType PlatformType_Cozeloop = "cozeloop"
const PlatformType PlatformType_Prompt = "prompt"
const PlatformType PlatformType_Evaluator = "evaluator"
const PlatformType PlatformType_EvaluationTarget =  "evaluation_target"
const PlatformType PlatformType_CozeBot = "coze_bot"
const PlatformType PlatformType_Project = "coze_project"
const PlatformType PlatformType_Workflow = "coze_workflow"
const PlatformType PlatformType_LoopAll = "loop_all"
const PlatformType PlatformType_InnerCozeloop = "inner_cozeloop"
const PlatformType PlatformType_InnerDoubao = "inner_doubao"
const PlatformType PlatformType_InnerPrompt = "inner_prompt"
const PlatformType PlatformType_InnerCozeBot = "inner_coze_bot"

typedef string SpanListType (ts.enum="true")
const SpanListType SpanListType_RootSpan = "root_span"
const SpanListType SpanListType_AllSpan = "all_span"
const SpanListType SpanListType_LlmSpan = "llm_span"

struct OrderBy {
    1: optional string field,
    2: optional bool is_asc,
}

struct UserInfo {
	1: optional string name
	2: optional string en_name
	3: optional string avatar_url
	4: optional string avatar_thumb
	5: optional string open_id
	6: optional string union_id
    8: optional string user_id
    9: optional string email
}

struct BaseInfo {
    1: optional UserInfo created_by
    2: optional UserInfo updated_by
    3: optional i64 created_at (api.js_conv='true', go.tag='json:"created_at"')
    4: optional i64 updated_at (api.js_conv='true', go.tag='json:"updated_at"')
}

typedef string ContentType(ts.enum="true")

const ContentType ContentType_Text = "Text" // 空间
const ContentType ContentType_Image = "Image"
const ContentType ContentType_Audio = "Audio"
const ContentType ContentType_MultiPart = "MultiPart"