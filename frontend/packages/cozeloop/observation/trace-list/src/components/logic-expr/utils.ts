// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-params */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { isArray, isEmpty, isNil } from 'lodash-es';
import { type ExprGroup } from '@cozeloop/components';
import {
  type FieldMeta,
  type GetTracesMetaInfoResponse,
  filter,
} from '@cozeloop/api-schema/observation';

const { QueryType } = filter;

export type FieldOptions =
  GetTracesMetaInfoResponse['field_metas'][number]['field_options'];

export type ValueType = FieldMeta['value_type'];
import { type LogicValue } from './logic-expr';
import {
  EMPTY_RENDER_CMP_OP_LIST,
  THREADS_STATUS_RECORDS,
  TimeUnit,
  FilterFields,
  THREADS_FEEDBACK_COZE_RECORDS,
} from './consts';
import {
  AUTO_EVAL_FEEDBACK_PREFIX,
  AUTO_EVAL_FEEDBACK,
  MANUAL_FEEDBACK,
  MANUAL_FEEDBACK_PREFIX,
} from './const';

const assignValueWithKind = <R>(params: { value: R; valueKind: string }) => {
  const { value, valueKind } = params;
  const defaultFieldValue = [];
  if (!value || (isArray(value) && value.length === 0)) {
    return defaultFieldValue;
  }

  if (valueKind === 'bool') {
    return [`${Boolean(value)}`];
  }

  if (value && Array.isArray(value)) {
    return value.map(item => String(item));
  }

  return [`${value}`];
};

export const getValueWithKind = (params: {
  value: string;
  valueKind: ValueType;
  fieldFilterType: string;
}) => [];

export const getOptionsWithKind = (params: {
  fieldOptions?: FieldOptions;
  valueKind?: ValueType;
}) => {
  const { fieldOptions, valueKind } = params;

  if (valueKind === 'bool' || valueKind === 'string') {
    return fieldOptions?.string_list ?? [];
  }

  if (valueKind === 'long') {
    return fieldOptions?.i64_list ?? [];
  }

  if (valueKind === 'double') {
    return fieldOptions?.f64_list ?? [];
  }

  return fieldOptions?.string_list ?? [];
};

export const formatExprValue = <L, O, R>(
  originValue?: LogicValue,
  tagFilterRecord?: Record<string, FieldMeta>,
  defaultImmutableKeys?: string[],
  ignoreKeys: string[] = [],
): ExprGroup<L, O, R> | undefined => {
  const { query_and_or, filter_fields, sub_filter } = originValue || {};

  if (!originValue || !filter_fields) {
    return undefined;
  }

  const exprOpNode: ExprGroup<L, O, R> = {
    logicOperator: query_and_or === 'or' ? 'or' : 'and',
    disableDeletion: Boolean(defaultImmutableKeys?.length),
    exprs: filter_fields
      .filter(
        fieldFilter =>
          !ignoreKeys.includes(
            fieldFilter.logic_field_name_type ?? fieldFilter.field_name,
          ),
      )
      .map(fieldFilter => {
        const { field_name, query_type, values } = fieldFilter || {};
        const leftValue = {
          value: field_name,
          type: field_name?.startsWith(AUTO_EVAL_FEEDBACK_PREFIX)
            ? AUTO_EVAL_FEEDBACK
            : field_name?.startsWith(MANUAL_FEEDBACK_PREFIX)
              ? MANUAL_FEEDBACK
              : (fieldFilter.logic_field_name_type ?? field_name),
          extraInfo: fieldFilter.extraInfo,
        };
        return {
          left: leftValue as L,
          operator: query_type as O,
          disableDeletion: defaultImmutableKeys?.includes(field_name ?? ''),
          right: values as R,
        };
      }),
  };

  if (sub_filter && sub_filter.length > 0) {
    exprOpNode.childExprGroups = [
      ...(exprOpNode.childExprGroups ?? []),
      ...sub_filter.map(
        child =>
          formatExprValue(
            child,
            tagFilterRecord,
            defaultImmutableKeys,
            ignoreKeys,
          ) as ExprGroup<L, O, R>,
      ),
    ];
  }
  return exprOpNode;
};

export const formatSpanFilterValue = <L, O, R>(
  originValue?: ExprGroup<L, O, R>,
  tagFilterRecord?: Record<string, FieldMeta>,
) => {
  if (!originValue) {
    return undefined;
  }

  const { logicOperator, exprs, childExprGroups } = originValue;

  const spanFilterNode: any = {
    query_and_or: logicOperator === 'or' ? 'or' : 'and',
    filter_fields: exprs?.map(item => {
      const left = item.left as {
        type: string;
        value: string;
        extraInfo?: Record<string, any>;
      };
      const fieldName = left?.value ?? left?.type;

      const valueKind =
        tagFilterRecord?.[left?.type as string]?.value_type ?? 'string';

      return {
        field_name:
          fieldName === AUTO_EVAL_FEEDBACK || fieldName === MANUAL_FEEDBACK
            ? ''
            : fieldName,
        logic_field_name_type: left?.type,
        extraInfo: left?.extraInfo,
        query_type: item.operator,
        values: assignValueWithKind<R>({
          value:
            item.operator === 'isNull' || item.operator === 'notNull'
              ? (true as R)
              : (item.right as R),
          valueKind,
        }),
      };
    }),
  };

  spanFilterNode.sub_filter = childExprGroups?.map(child =>
    formatSpanFilterValue(child, tagFilterRecord),
  );

  return spanFilterNode;
};

export const getFilteredValue = (originValue?: any): any | undefined => {
  const { filter_fields, sub_filter } = originValue || {};
  if (!originValue || !filter_fields) {
    return undefined;
  }

  const checkValueEmpty = (fieldFilterType: string, filterValue: any) =>
    EMPTY_RENDER_CMP_OP_LIST.includes(fieldFilterType)
      ? false
      : Object.values(filterValue).every(value => isNil(value));

  originValue.filter_fields = filter_fields.filter(tagFilter => {
    const { field_name, query_type, values } = tagFilter || {};
    return field_name && query_type && !checkValueEmpty(query_type, values);
  });

  if (sub_filter && sub_filter.length > 0) {
    originValue.sub_filter = sub_filter
      .map(spanFilter => getFilteredValue(spanFilter))
      .filter(Boolean) as any[];
  }

  return originValue;
};

export const getKeyCopywriting = (key: string) => {
  const snakeToPascalCase = (str: string) => {
    const specialWords: { [key: string]: string } = {
      id: 'ID',
      psm: 'PSM',
    };

    return str
      .split('_')
      .map(word => {
        if (specialWords[word.toLowerCase()]) {
          return specialWords[word.toLowerCase()];
        }
        return word.charAt(0).toUpperCase() + word.slice(1).toLowerCase();
      })
      .join('');
  };

  switch (key) {
    case FilterFields.BIZ_ID:
      return 'MessageID';
    case FilterFields.BOT_ID:
      return 'BotName';
    case FilterFields.APP_ID:
      return 'AppName';
    case FilterFields.FEEDBACK:
      return 'Feedback-自动评测';
    case FilterFields.FEEDBACK_MANUAL:
      return 'Feedback-人工标注';
    case FilterFields.FEEDBACK_COZE:
      return 'Feedback-Coze 对话';
    case FilterFields.WORKFLOW_ID:
      return 'WorkflowName';
    default:
      return snakeToPascalCase(key);
  }
};

export const getOptionCopywriting = (key: string, option: string | number) => {
  switch (key) {
    case FilterFields.STATUS_KEY:
      return THREADS_STATUS_RECORDS[option]?.label;
    case FilterFields.FEEDBACK_COZE:
      return THREADS_FEEDBACK_COZE_RECORDS[option]?.label;
    default:
      return option;
  }
};

export const getLabelUnit = (key: string) => {
  switch (key) {
    case FilterFields.DURATION:
    case FilterFields.LATENCY_FIRST_RESP:
    case FilterFields.START_TIME_FIRST_RESP:
    case FilterFields.LATENCY:
      return TimeUnit.MS;
    default:
      return undefined;
  }
};

export const checkFilterHasEmpty = (filters?: LogicValue) =>
  filters?.filter_fields?.length === 0 ||
  filters?.filter_fields?.some(
    item =>
      isEmpty(item.values) &&
      item.query_type !== QueryType.Exist &&
      item.query_type !== QueryType.NotExist,
  );

export const checkFilterAllEmpty = (filters?: LogicValue) =>
  !filters?.filter_fields?.length ||
  filters?.filter_fields?.every(
    item =>
      isEmpty(item.values) &&
      item.query_type !== QueryType.Exist &&
      item.query_type !== QueryType.NotExist,
  );
