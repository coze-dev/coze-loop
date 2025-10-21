namespace go coze.loop.evaluation.domain_openapi.eval_target

struct CreateEvalTargetParam {
    1: optional string source_target_id
    2: optional string source_target_version
    3: optional EvalTargetType eval_target_type
    4: optional CozeBotInfoType bot_info_type
    5: optional string bot_publish_version // 如果是发布版本则需要填充这个字段
}

enum EvalTargetType {
    CozeBot = 1 // CozeBot
    CozeLoopPrompt = 2 // Prompt
    Trace = 3 // Trace
    CozeWorkflow = 4
    VolcengineAgent = 5 // 火山智能体
}

enum CozeBotInfoType {
   DraftBot = 1 // 草稿 bot
   ProductBot = 2 // 商店 bot
}