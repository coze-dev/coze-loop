// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export interface Trajectory {
  /** trace_id */
  id?: string,
  /** 根节点，记录整个轨迹的信息 */
  root_step?: RootStep,
  /** agent step列表，记录轨迹中agent执行信息 */
  agent_steps?: AgentStep[],
}
export interface RootStep {
  /** 唯一ID，trace导入时取span_id */
  id?: string,
  /** name，trace导入时取span_name */
  name?: string,
  /** 输入 */
  input?: string,
  /** 输出 */
  output?: string,
  /**
   * 系统属性
   * 保留字段，可以承载业务自定义的属性
  */
  metadata?: {
    [key: string | number]: string
  },
  basic_info?: BasicInfo,
  metrics_info?: MetricsInfo,
}
export interface AgentStep {
  /**
   * 基础属性
   * 唯一ID，trace导入时取span_id
  */
  id?: string,
  /** 父ID， trace导入时取parent_span_id */
  parent_id?: string,
  /** name，trace导入时取span_name */
  name?: string,
  /** 输入 */
  input?: string,
  /** 输出 */
  output?: string,
  /** 子节点，agent执行内部经历了哪些步骤 */
  steps?: Step[],
  /**
   * 系统属性
   * 保留字段，可以承载业务自定义的属性
  */
  metadata?: {
    [key: string | number]: string
  },
  basic_info?: BasicInfo,
  metrics_info?: MetricsInfo,
}
export interface Step {
  /**
   * 基础属性
   * 唯一ID，trace导入时取span_id
  */
  id?: string,
  /** 父ID， trace导入时取parent_span_id */
  parent_id?: string,
  /** 类型 */
  type?: StepType,
  /** name，trace导入时取span_name */
  name?: string,
  /** 输入 */
  input?: string,
  /** 输出 */
  output?: string,
  /**
   * 各种类型补充信息
   * type=model时填充
  */
  model_info?: ModelInfo,
  /**
   * 系统属性
   * 保留字段，可以承载业务自定义的属性
  */
  metadata?: {
    [key: string | number]: string
  },
  basic_info?: BasicInfo,
}
export enum StepType {
  Agent = "agent",
  Model = "model",
  Tool = "tool",
}
export interface ModelInfo {
  input_tokens?: number,
  output_tokens?: number,
  /** 首包耗时，单位毫秒 */
  latency_first_resp?: string,
  reasoning_tokens?: number,
  input_read_cached_tokens?: number,
  input_creation_cached_tokens?: number,
}
export interface BasicInfo {
  /** 单位毫秒 */
  started_at?: string,
  /** 单位毫秒 */
  duration?: string,
  error?: Error,
}
export interface Error {
  code?: number,
  msg?: string,
}
export interface MetricsInfo {
  /** 单位毫秒 */
  llm_duration?: string,
  /** 单位毫秒 */
  tool_duration?: string,
  /** Tool错误分布，格式为：错误码-->list<ToolStepID> */
  tool_errors?: {
    [key: string | number]: string[]
  },
  /** Tool错误率 */
  tool_error_rate?: number,
  /** Model错误分布，格式为：错误码-->list<ModelStepID> */
  model_errors?: {
    [key: string | number]: string[]
  },
  /** Model错误率 */
  model_error_rate?: number,
  /** Tool Step占比(分母是总子Step) */
  tool_step_proportion?: number,
  /** 输入token数 */
  input_tokens?: number,
  /** 输出token数 */
  output_tokens?: number,
}