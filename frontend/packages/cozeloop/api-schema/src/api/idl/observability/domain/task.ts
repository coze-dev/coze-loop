// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as export_dataset from './export_dataset';
export { export_dataset };
import * as filter from './filter';
export { filter };
import * as common from './common';
export { common };
export enum TimeUnit {
  Day = "day",
  Week = "week",
  Null = "null",
}
export enum TaskType {
  AutoEval = "auto_evaluate",
  /** 自动评测 */
  AutoDataReflow = "auto_data_reflow",
}
/** 数据回流 */
export enum TaskRunType {
  BackFill = "back_fill",
  /** 历史数据回填 */
  NewData = "new_data",
}
/** 新数据 */
export enum TaskStatus {
  Unstarted = "unstarted",
  /** 未启动 */
  Running = "running",
  /** 正在运行 */
  Failed = "failed",
  /** 失败 */
  Success = "success",
  /** 成功 */
  Pending = "pending",
  /** 中止 */
  Disabled = "disabled",
}
/** 禁用 */
export enum RunStatus {
  Running = "running",
  /** 正在运行 */
  Done = "done",
}
/** 完成运行 */
export enum TaskSource {
  User = "user",
  /** 用户创建 */
  Workflow = "workflow",
}
/**
 * 工作流创建
 * Task
*/
export interface Task {
  /** 任务 id */
  id?: string,
  /** 名称 */
  name: string,
  /** 描述 */
  description?: string,
  /** 所在空间 */
  workspace_id?: string,
  /** 类型 */
  task_type: TaskType,
  /** 状态 */
  task_status?: TaskStatus,
  /** 规则 */
  rule?: Rule,
  /** 配置 */
  task_config?: TaskConfig,
  /** 任务状态详情 */
  task_detail?: RunDetail,
  /** 任务历史数据执行详情 */
  backfill_task_detail?: RunDetail,
  /** 创建来源 */
  task_source?: TaskSource,
  /** 基础信息 */
  base_info?: common.BaseInfo,
}
/** Rule */
export interface Rule {
  /** Span 过滤条件 */
  span_filters?: filter.SpanFilterFields,
  /** 采样配置 */
  sampler?: Sampler,
  /** 生效时间窗口 */
  effective_time?: EffectiveTime,
  /** 历史数据生效时间窗口 */
  backfill_effective_time?: EffectiveTime,
}
export interface Sampler {
  /** 采样率 */
  sample_rate?: number,
  /** 采样上限 */
  sample_size?: number,
  /** 是否启动任务循环 */
  is_cycle?: boolean,
  /** 采样单次上限 */
  cycle_count?: number,
  /** 循环间隔 */
  cycle_interval?: number,
  /** 循环时间单位 */
  cycle_time_unit?: TimeUnit,
}
export interface EffectiveTime {
  /** ms timestamp */
  start_at?: string,
  /** ms timestamp */
  end_at?: string,
}
/** TaskConfig */
export interface TaskConfig {
  /** 配置的评测规则信息 */
  auto_evaluate_configs?: AutoEvaluateConfig[],
  /** 配置的数据回流的数据集信息 */
  data_reflow_config?: DataReflowConfig[],
}
export interface DataReflowConfig {
  /** 数据集id，新增数据集时可为空 */
  dataset_id?: string,
  /** 数据集名称 */
  dataset_name?: string,
  /** 数据集列数据schema */
  dataset_schema?: export_dataset.DatasetSchema,
  field_mappings?: export_dataset.FieldMapping[],
}
export interface AutoEvaluateConfig {
  evaluator_version_id: string,
  evaluator_id: string,
  field_mappings: EvaluateFieldMapping[],
}
/** RunDetail */
export interface RunDetail {
  success_count?: number,
  failed_count?: number,
  total_count?: number,
}
export interface BackfillDetail {
  success_count?: number,
  failed_count?: number,
  total_count?: number,
  backfill_status?: RunStatus,
  last_span_page_token?: string,
}
export interface EvaluateFieldMapping {
  /** 数据集字段约束 */
  field_schema: export_dataset.FieldSchema,
  trace_field_key: string,
  trace_field_jsonpath: string,
  eval_set_name?: string,
}
/** TaskRun */
export interface TaskRun {
  /** 任务 run id */
  id: string,
  /** 所在空间 */
  workspace_id: string,
  /** 任务 id */
  task_id: string,
  /** 类型 */
  task_type: TaskRunType,
  /** 状态 */
  run_status: RunStatus,
  /** 任务状态详情 */
  run_detail?: RunDetail,
  /** 任务历史数据执行详情 */
  backfill_run_detail?: BackfillDetail,
  run_start_at: string,
  run_end_at: string,
  /** 配置 */
  task_run_config?: TaskRunConfig,
  /** 基础信息 */
  base_info?: common.BaseInfo,
}
export interface TaskRunConfig {
  /** 自动评测对应的运行配置信息 */
  auto_evaluate_run_config?: AutoEvaluateRunConfig,
  /** 数据回流对应的运行配置信息 */
  data_reflow_run_config?: DataReflowRunConfig,
}
export interface AutoEvaluateRunConfig {
  expt_id: string,
  expt_run_id: string,
  eval_id: string,
  schema_id: string,
  schema?: string,
  end_at: string,
  cycle_start_at: string,
  cycle_end_at: string,
  status: string,
}
export interface DataReflowRunConfig {
  dataset_id: string,
  dataset_run_id: string,
  end_at: string,
  cycle_start_at: string,
  cycle_end_at: string,
  status: string,
}