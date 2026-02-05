// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { EvaluatorType } from '@cozeloop/api-schema/evaluation';

export const SCROLL_DELAY = 200;
export const SCROLL_OFFSET = 200;
export const EVALUATOR_CODE_DOCUMENT_LINK =
  'https://loop.coze.cn/open/docs/cozeloop/create_evaluators';

/*
 * 获取评估器类型的文本表示
 * @param type 评估器类型
 * @returns 评估器类型的文本表示
 */
export function getEvaluatorTypeText(type: EvaluatorType | undefined) {
  return type === EvaluatorType.Code ? 'Code' : 'LLM';
}
