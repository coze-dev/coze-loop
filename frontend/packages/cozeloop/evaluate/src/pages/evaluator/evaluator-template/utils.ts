// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type Evaluator,
  type EvaluatorFilterCondition,
  EvaluatorFilterLogicOp,
  EvaluatorFilterOperatorType,
  type EvaluatorFilterOption,
  type EvaluatorFilters,
  EvaluatorTagKey,
} from '@cozeloop/api-schema/evaluation';

import { type ListTemplatesParams, type TemplateFilter } from './types';

/** 筛选逻辑转换为接口格式 */
function filterTransform(
  filters: TemplateFilter | undefined,
): EvaluatorFilters {
  const conditions: EvaluatorFilterCondition[] = [];
  Object.entries(filters ?? {}).forEach(([key, value]) => {
    const tagKey = key as EvaluatorTagKey;
    let filterVal: string = value;
    if (Array.isArray(value)) {
      filterVal = value.join(',');
    }
    // 空字符串认为是空条件，不参与筛选
    if (filterVal === '') {
      return;
    }
    const newCondition: EvaluatorFilterCondition = {
      tag_key: tagKey,
      operator: EvaluatorFilterOperatorType.In,
      value: filterVal,
    };
    conditions.push(newCondition);
  });
  const evaluatorFilters: EvaluatorFilters = {
    logic_op: EvaluatorFilterLogicOp.And,
    filter_conditions: conditions,
  };
  return evaluatorFilters;
}

/** 转换模板筛选器为评估器筛选器请求的参数 */
export function evaluatorFilterTransform(
  params: ListTemplatesParams | undefined,
): EvaluatorFilterOption {
  const filter: EvaluatorFilterOption = {
    search_keyword: params?.search_keyword || undefined,
    filters: filterTransform(params?.filters),
  };
  return filter;
}

/** 从评估器的tags转换为字符串数组 */
export function evaluatorTagsTransformer(tags?: Evaluator['tags']): string[] {
  if (!tags) {
    return [];
  }
  const tagObj = Object.values(tags).flat()?.[0] || {};
  // 不展示Name标签
  delete tagObj.Name;
  const arr: string[] = [];
  arr.push(...(tagObj[EvaluatorTagKey.Category] || []));
  arr.push(...(tagObj[EvaluatorTagKey.TargetType] || []));
  arr.push(...(tagObj[EvaluatorTagKey.Objective] || []));
  arr.push(...(tagObj[EvaluatorTagKey.BusinessScenario] || []));

  return arr;
}

export function getEvaluatorMaxTagCount(tagsArr: string[]): {
  total: number;
  showCount: number;
} {
  const total = tagsArr.length;
  // 将每个tag长度累加，直到超过16个字符串长度
  let textLength = 0;
  let showCount = 0;
  for (const tag of tagsArr) {
    for (let i = 0; i < tag.length; i++) {
      if (tag.charCodeAt(i) > 127) {
        textLength += 2;
      } else {
        textLength += 1;
      }
    }
    if (textLength > 32) {
      break;
    }
    showCount++;
  }

  return {
    total,
    showCount,
  };
}

export const getEvaluatorTagList = (
  tags?: Evaluator['tags'],
): {
  // coze和semi类型不兼容，先any处理
  tagList: {
    color: string;
    children: string;
    size: string;
    shape: string;
  }[];
  maxTagCount: number;
} => {
  const strArr = evaluatorTagsTransformer(tags);
  const { showCount } = getEvaluatorMaxTagCount(strArr);

  const tagList = strArr.map(tag => ({
    color: 'primary',
    children: tag,
    size: 'small',
    shape: 'square',
  }));

  return {
    tagList,
    maxTagCount: showCount,
  };
};
