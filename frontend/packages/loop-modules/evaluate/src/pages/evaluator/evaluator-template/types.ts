// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type EvaluatorTagKey } from '@cozeloop/api-schema/evaluation';

/** 评估器模板筛选参数(前端) */
export interface TemplateFilter {
  /** 评估器类型 */
  [EvaluatorTagKey.Category]?: string[];
  /** 评估对象 */
  [EvaluatorTagKey.TargetType]?: string[];
  /** 评估目标 */
  [EvaluatorTagKey.Objective]?: string[];
  /** 业务场景 */
  [EvaluatorTagKey.BusinessScenario]?: string[];
}

export enum EvaluatorTypeTagText {
  Prompt = 'LLM',
  Code = 'Code',
}

/** 评估器模板列表查询参数(前端) */
export interface ListTemplatesParams {
  workspace_id: string;
  search_keyword?: string;
  filters?: TemplateFilter;
  page_size?: number;
  page_number?: number;
}

export enum EvaluatorSource {
  CUSTOM = 'custom',
  BUILTIN = 'builtin',
}
