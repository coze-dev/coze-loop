// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */

/* eslint-disable @typescript-eslint/no-explicit-any */
import { memo, useCallback, useMemo, useState } from 'react';

import { type FieldMeta } from '@cozeloop/api-schema/observation';
import { type OptionProps, type SelectProps } from '@coze-arch/coze-design';

import {
  type ExprGroup,
  LogicExpr,
  type LogicOperator,
} from '@/shared/components/logic-expr';

import {
  formatExprValue,
  formatSpanFilterValue,
  getKeyCopywriting,
} from './utils';
import { RightRender } from './right-render';
import { OperatorRenderer } from './operator-render';
import { LeftRenderer } from './left-render';
import { ErrorMsgRender } from './error-msg-render';

import styles from './index.module.less';

type LogicExprTypes = [
  Left | undefined,
  number | undefined | string,
  string | number | string[] | number[] | undefined,
];

// 自定义右侧渲染器的映射类型
export type CustomRightRenderMap = Record<
  string,
  (props?: SelectProps & Record<string, any>) => React.ReactNode
>;

export interface LogicValue {
  filter_fields: LogicItem[];
  query_and_or?: string;
  sub_filter?: Array<LogicValue>;
}

export interface LogicItem {
  field_name: string;
  query_type: string;
  values: string[];
  logic_field_name_type?: string;
  extraInfo?: Record<string, any>;
  extra_info?: Record<string, any>;
  field_type?: string;
  is_custom?: boolean;
}

export interface AnalyticsLogicExprProps {
  disabled?: boolean;
  value?: LogicValue;
  disableDuplicateSelect?: boolean;
  defaultImmutableKeys?: string[];
  // 新增：禁用行的 key 列表，这些行的所有操作都将被禁用，包括删除
  disabledRowKeys?: string[];
  tagFilterRecord: Record<string, FieldMeta>;
  onChange?: (value?: LogicValue) => void;
  allowLogicOperators?: LogicOperator[];
  invalidateExpr?: Set<string>;
  // 新增的自定义渲染器
  customRightRenderMap?: CustomRightRenderMap;
  customLeftRenderMap?: CustomRightRenderMap;
  ignoreKeys?: string[];
}

// Helper function to sort strings by first character (letters first)
const sortByFirstChar = (a: string, b: string): number => {
  const isLetter = (char: string) => /^[A-Za-z]$/.test(char);
  const aIsLetter = isLetter(a.charAt(0));
  const bIsLetter = isLetter(b.charAt(0));

  if (aIsLetter && bIsLetter) {
    return a.localeCompare(b, undefined, { sensitivity: 'base' });
  }
  return aIsLetter ? -1 : bIsLetter ? 1 : 0;
};

export interface Left {
  type: string | undefined;
  value: string | undefined;
  extra_info?: Record<string, any>;
  extraInfo?: Record<string, any>;
}

const defaultValue = {
  left: undefined,
  operator: undefined,
  right: undefined,
};

export const AnalyticsLogicExpr = memo((props: AnalyticsLogicExprProps) => {
  const {
    value,
    tagFilterRecord,
    disableDuplicateSelect,
    defaultImmutableKeys,
    disabledRowKeys,
    onChange,
    disabled,
    allowLogicOperators = ['and'],
    invalidateExpr = new Set(),
    customRightRenderMap = {},
    customLeftRenderMap = {},
    ignoreKeys = [],
  } = props;

  const exprValue = useMemo(
    () =>
      formatExprValue<LogicExprTypes[0], LogicExprTypes[1], LogicExprTypes[2]>(
        value,
        tagFilterRecord,
        defaultImmutableKeys,
        disabledRowKeys,
        ignoreKeys,
      ),
    [value, defaultImmutableKeys, disabledRowKeys, tagFilterRecord, ignoreKeys],
  );

  const checkIsInvalidateExpr = useCallback(
    (expr: Left | undefined) =>
      expr ? invalidateExpr.has(expr.value ?? '') : false,
    [invalidateExpr],
  );

  const [valueChangeMap, setValueChangeMap] = useState<Record<string, boolean>>(
    {},
  );

  const { tagLeftOption } = useMemo<{
    tagLeftOption: OptionProps[];
  }>(() => {
    const selectedItemKeyList =
      exprValue?.exprs?.map(item => item.left?.type) || [];
    return {
      tagLeftOption: Object.keys(tagFilterRecord)
        .filter(key => !ignoreKeys.includes(key))
        .sort((a, b) => sortByFirstChar(a, b))
        .map(key => ({
          label: getKeyCopywriting(key),
          value: key,
          disabled:
            (disableDuplicateSelect && selectedItemKeyList?.includes(key)) ||
            disabledRowKeys?.includes(key),
        })),
    };
  }, [
    exprValue,
    disableDuplicateSelect,
    ignoreKeys,
    tagFilterRecord,
    disabledRowKeys,
  ]);

  const handleValueChangeStatus = useCallback(
    (fieldName: string, changed: boolean) => {
      setValueChangeMap(prev => ({
        ...prev,
        [fieldName]: changed,
      }));
    },
    [setValueChangeMap],
  );

  const handleLogicExprChange = useCallback(
    (
      expr:
        | ExprGroup<
            Left | undefined,
            string | number | undefined,
            string | number | string[] | number[] | undefined
          >
        | undefined,
    ) => {
      const result = formatSpanFilterValue(expr, tagFilterRecord);
      onChange?.(result);
    },
    [onChange, tagFilterRecord],
  );

  return (
    <LogicExpr<LogicExprTypes[0], LogicExprTypes[1], LogicExprTypes[2]>
      value={exprValue}
      readonly={disabled}
      allowLogicOperators={allowLogicOperators}
      onDeleteExpr={key => {
        setValueChangeMap(prev => ({
          ...prev,
          [key?.value as string]: false,
        }));
      }}
      exprGroupRenderContentItemsClassName={
        styles['expr-group-render-content-items']
      }
      leftRender={leftRenderProps => (
        <LeftRenderer
          expr={leftRenderProps.expr}
          onExprChange={leftRenderProps.onExprChange}
          tagLeftOption={tagLeftOption}
          disabled={disabled}
          defaultImmutableKeys={defaultImmutableKeys}
          disabledRowKeys={disabledRowKeys}
          checkIsInvalidateExpr={checkIsInvalidateExpr}
          customLeftRenderMap={customLeftRenderMap}
        />
      )}
      operatorRender={operatorRenderProps => (
        <OperatorRenderer
          expr={operatorRenderProps.expr}
          onExprChange={operatorRenderProps.onExprChange}
          tagFilterRecord={tagFilterRecord}
          disabled={disabled}
          defaultImmutableKeys={defaultImmutableKeys}
          disabledRowKeys={disabledRowKeys}
          checkIsInvalidateExpr={checkIsInvalidateExpr}
        />
      )}
      rightRender={rightRenderProps => {
        const {
          expr: { left, operator, right },
          onChange: onRightValueChange,
        } = rightRenderProps;

        const isInvalidateExpr = checkIsInvalidateExpr(left);

        return (
          <RightRender
            left={left}
            operator={operator}
            right={right}
            disabled={disabled}
            defaultImmutableKeys={defaultImmutableKeys}
            disabledRowKeys={disabledRowKeys}
            isInvalidateExpr={isInvalidateExpr}
            valueChanged={valueChangeMap[left?.value ?? '']}
            tagFilterRecord={tagFilterRecord}
            onRightValueChange={onRightValueChange}
            onValueChangeStatus={handleValueChangeStatus}
            customRightRenderMap={customRightRenderMap}
          />
        );
      }}
      errorMsgRender={errorMsgRenderProps => (
        <ErrorMsgRender
          expr={errorMsgRenderProps.expr}
          tagLeftOption={tagLeftOption}
          checkIsInvalidateExpr={checkIsInvalidateExpr}
          valueChangeMap={valueChangeMap}
        />
      )}
      maxNestingDepth={1}
      defaultExpr={defaultValue}
      onChange={handleLogicExprChange}
    />
  );
});
