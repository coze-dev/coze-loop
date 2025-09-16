// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  FieldType,
  type FilterCondition,
  FilterLogicOp,
  FilterOperatorType,
  type Filters,
} from '@cozeloop/api-schema/evaluation';

import { type LogicFilter } from '../components/logic-editor';

/** 操作符映射 */
const operatorMap: Record<string, FilterOperatorType> = {
  equals: FilterOperatorType.Equal,
  'not-equals': FilterOperatorType.NotEqual,
  contains: FilterOperatorType.In,
  'not-contains': FilterOperatorType.NotIn,
  'greater-than': FilterOperatorType.Greater,
  'greater-than-equals': FilterOperatorType.GreaterOrEqual,
  'less-than': FilterOperatorType.Less,
  'less-than-equals': FilterOperatorType.LessOrEqual,
  like: FilterOperatorType.Like,
  'not-like': FilterOperatorType.NotLike,
};

/**
 * 逻辑筛选数据转换为评测接口支持的筛选条件
 * - left: 字段, 约定是一个json字符串，里面包含字段类型和字段key
 * - operator: 操作符
 * - right: 操作值
 * @param logicFilter 逻辑筛选数据
 */
function logicFilterToCondition(logicFilter: LogicFilter | undefined): Filters {
  if (!logicFilter) {
    return {
      logic_op: FilterLogicOp.And,
      filter_conditions: [],
    };
  }
  const { logicOperator, exprs } = logicFilter;
  // 整体逻辑操作符：与、或, 默认为与
  const logicOpt =
    logicOperator === 'or' ? FilterLogicOp.Or : FilterLogicOp.And;

  const filters: Filters = {
    logic_op: logicOpt,
    filter_conditions: exprs
      ?.map(expr => {
        const { left, operator, right } = expr;
        const opt = operatorMap[operator];
        // 这里后段约定value必须是字符串，所以需要转换一下
        let value = right?.toString?.();
        if (Array.isArray(right)) {
          value = right.join(',');
        }
        const fieldName = Array.isArray(left) ? left[left.length - 1] : left;
        const field = JSON.parse(fieldName) as { type: FieldType; key: string };
        const filterCondition: FilterCondition = {
          field: {
            field_type: field.type,
            field_key: field.key,
          },
          operator: opt,
          value,
        };
        if (field.type === FieldType.SourceTarget) {
          filterCondition.source_target = {
            source_target_ids: right?.evalTargetId || [],
            eval_target_type: right?.type || '',
          };
          // 这里之前是 delete filterCondition.value, 但是有类型问题，所以改成这样
          filterCondition.value = '';
        }
        return filterCondition;
      })
      .filter(Boolean),
  };
  return filters;
}

/** 普通对象类型的筛选数据转化为筛选条件 */
function filterToCondition<T extends object>(
  filter: T,
  filterFields: {
    key: keyof T;
    type: FieldType;
    operator?: FilterOperatorType;
  }[],
): FilterCondition[] {
  const conditions: FilterCondition[] = [];
  filterFields?.forEach(field => {
    const val = filter[field.key];
    // 如果值为空，则不添加筛选条件
    if (
      val === undefined ||
      val === null ||
      val === '' ||
      (Array.isArray(val) && val.length === 0)
    ) {
      return;
    }

    let value = val?.toString?.() ?? '';
    if (Array.isArray(val)) {
      value = val.join(',');
    }
    // 这里后段约定value必须是字符串，所以需要转换一下
    conditions.push({
      field: { field_type: field.type },
      operator: field.operator ?? FilterOperatorType.In,
      value,
    });
  });
  return conditions;
}

/**
 * 将前端筛选数据格式化为评测接口支持的筛选条件
 * @param logicFilter 逻辑筛选数据
 * @param filter 普通对象类型的筛选数据
 */
export function filterToFilters<T extends object>({
  logicFilter,
  filter,
  filterFields,
}: {
  logicFilter?: LogicFilter | undefined;
  filter?: T;
  filterFields?: { key: keyof T; type: FieldType }[];
}) {
  const filters: Filters = logicFilterToCondition(logicFilter);
  if (filter && filterFields) {
    const conditions = filterToCondition(filter, filterFields);
    filters.filter_conditions?.push(...conditions);
  }
  return filters;
}

/**
 * 获取逻辑筛选器字段名称
 * @param fieldType 字段类型
 * @param key 字段key
 */
export function getLogicFieldName(fieldType: FieldType, key: string | number) {
  return JSON.stringify({ type: fieldType, key });
}
