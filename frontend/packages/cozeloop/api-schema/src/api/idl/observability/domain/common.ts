export enum PlatformType {
  Cozeloop = "cozeloop",
  Prompt = "prompt",
  Evaluator = "evaluator",
  EvaluationTarget = "evaluation_target",
  CozeBot = "coze_bot",
  Project = "coze_project",
  Workflow = "coze_workflow",
  Ark = "ark",
  VeADK = "veadk",
  VeAgentkit = "ve_agentkit",
  LoopAll = "loop_all",
  InnerCozeloop = "inner_cozeloop",
  InnerDoubao = "inner_doubao",
  InnerPrompt = "inner_prompt",
  InnerCozeBot = "inner_coze_bot",
}
export enum SpanListType {
  RootSpan = "root_span",
  AllSpan = "all_span",
  LlmSpan = "llm_span",
}
export interface OrderBy {
  field?: string,
  is_asc?: boolean,
}
export interface UserInfo {
  name?: string,
  en_name?: string,
  avatar_url?: string,
  avatar_thumb?: string,
  open_id?: string,
  union_id?: string,
  user_id?: string,
  email?: string,
}
export interface BaseInfo {
  created_by?: UserInfo,
  updated_by?: UserInfo,
  created_at?: string,
  updated_at?: string,
}
export enum ContentType {
  Text = "Text",
  /** 空间 */
  Image = "Image",
  Audio = "Audio",
  MultiPart = "MultiPart",
}
export interface Session {
  user_id?: string
}