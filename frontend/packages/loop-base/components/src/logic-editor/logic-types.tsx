// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import {
  CozInputNumber,
  DatePicker,
  Input,
  Select,
  TextArea,
} from '@coze-arch/coze-design';

import { useI18n, type I18nType } from '@/provider';

import { type Expr, type ExprGroup } from '../logic-expr';

export interface LogicOperation {
  label: string;
  value: string;
}

export type LogicFilterLeft = string | string[];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type LogicFilter = ExprGroup<LogicFilterLeft, string, any>;

export interface RenderProps {
  disabled?: boolean;
  fields: LogicField[];
  /** 开启级联模式，佐治会变成数组 */
  enableCascadeMode?: boolean;
}

/** 逻辑编辑器的字段 */
export interface LogicField {
  /** 字段标题 */
  title: React.ReactNode;
  /** 字段名称 */
  name: string;
  /** 字段类型 */
  type: 'string' | 'number' | 'options' | 'custom';
  /* 自定义操作符右边的输入编辑器的属性，例如给下拉框传递optionList */
  setterProps?: Record<string, unknown>;
  /** 自定义操作符右边的输入编辑器 */
  setter?: LogicSetter;
  /** 禁用操作符列表 */
  disabledOperations?: string[];
  /** operator 自定义属性 */
  operatorProps?: Record<string, unknown>;
  /** 自定义操作符列表，会覆盖原有列表 */
  customOperations?: LogicOperation[];
  /** 子字段 */
  children?: LogicField[];
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface DataTypeSetterProps<T = any> {
  [key: string]: unknown;
  value: T;
  expr: Expr | undefined;
  field: LogicField;
  disabled: boolean;
  onChange: (val: T) => void;
}

export type LogicSetter = (props: DataTypeSetterProps) => JSX.Element | null;

export interface LogicDataType {
  type: 'string' | 'number' | 'date' | 'options';
  operations: LogicOperation[];
  setter: LogicSetter;
}

export const baseOperations = (i18n: I18nType): LogicOperation[] => [
  {
    label: i18n.t('contain'),
    value: 'contains',
  },
  {
    label: i18n.t('not_contain'),
    value: 'not-contains',
  },
  {
    label: i18n.t('equal_to'),
    value: 'equals',
  },
  {
    label: i18n.t('not_equal_to'),
    value: 'not-equals',
  },
];

export const stringOperations = (i18n: I18nType): LogicOperation[] => [
  // 注意：字符串类型的包含不包含和选项类的包含不包含枚举值不同，需要like模式
  {
    label: i18n.t('contain'),
    value: 'like',
  },
  {
    label: i18n.t('not_contain'),
    value: 'not-like',
  },
  {
    label: i18n.t('equal_to'),
    value: 'equals',
  },
  {
    label: i18n.t('not_equal_to'),
    value: 'not-equals',
  },
];

export const numberOperations = (i18n: I18nType): LogicOperation[] => [
  {
    label: i18n.t('equal_to'),
    value: 'equals',
  },
  {
    label: i18n.t('not_equal_to'),
    value: 'not-equals',
  },
  {
    label: i18n.t('greater_than'),
    value: 'greater-than',
  },
  {
    label: i18n.t('task_filter_gte'),
    value: 'greater-than-equals',
  },
  {
    label: i18n.t('less_than'),
    value: 'less-than',
  },
  {
    label: i18n.t('task_filter_lte'),
    value: 'less-than-equals',
  },
];

export const dateOperations = (i18n: I18nType): LogicOperation[] => [
  {
    label: i18n.t('equal_to'),
    value: 'equals',
  },
  {
    label: i18n.t('not_equal_to'),
    value: 'not-equals',
  },
  {
    label: i18n.t('later_than'),
    value: 'greater-than',
  },
  {
    label: i18n.t('earlier_than'),
    value: 'less-than',
  },
];

export const selectOperations = (i18n: I18nType): LogicOperation[] => [
  {
    label: i18n.t('contain'),
    value: 'contains',
  },
  {
    label: i18n.t('not_contain'),
    value: 'not-contains',
  },
];

function StringSetter({
  /** 默认为多行文本模式 */
  textAreaMode = true,
  ...props
}: DataTypeSetterProps<string> & { textAreaMode?: boolean }) {
  const I18n = useI18n();
  if (textAreaMode === false) {
    return <Input placeholder={I18n.t('please_enter')} {...props} />;
  }
  return <TextArea placeholder={I18n.t('please_enter')} rows={1} {...props} />;
}

function NumberSetter(props: DataTypeSetterProps<number>) {
  const I18n = useI18n();
  const { value, onChange, ...rest } = props;
  return (
    <CozInputNumber
      placeholder={I18n.t('please_enter')}
      {...rest}
      className={`w-full ${(props as { className?: string }).className ?? ''}`}
      value={value ?? ''}
      onChange={onChange as (val: number | string) => void}
    />
  );
}
function DateSetter(props: DataTypeSetterProps<string>) {
  const { value, onChange, ...rest } = props;
  return (
    <DatePicker
      {...rest}
      value={value}
      onChange={val => onChange(val as string)}
    />
  );
}

function SelectSetter(
  props: DataTypeSetterProps<string> & {
    className?: string;
    optionList?: { label: string; value: string }[];
  },
) {
  const I18n = useI18n();
  const { value, onChange, optionList = [], className = '', ...rest } = props;
  return (
    <Select
      placeholder={I18n.t('please_select')}
      {...rest}
      className={`w-full ${className}`}
      optionList={optionList}
      value={value}
      onChange={val => onChange(val as string)}
    />
  );
}

export function useDataTypeList() {
  const I18n = useI18n();

  const [state] = useState(() => {
    const dataTypeList: LogicDataType[] = [
      {
        type: 'string',
        operations: stringOperations(I18n),
        setter: StringSetter,
      },
      {
        type: 'number',
        operations: numberOperations(I18n),
        setter: NumberSetter as unknown as LogicSetter,
      },
      {
        type: 'date',
        operations: dateOperations(I18n),
        setter: DateSetter,
      },
      {
        type: 'options',
        operations: selectOperations(I18n),
        setter: SelectSetter,
      },
    ];

    return dataTypeList;
  });
  return state;
}
// export const dataTypeList: LogicDataType[] = [
//   {
//     type: 'string',
//     operations: stringOperations,
//     setter: StringSetter,
//   },
//   {
//     type: 'number',
//     operations: numberOperations,
//     setter: NumberSetter as unknown as LogicSetter,
//   },
//   {
//     type: 'date',
//     operations: dateOperations,
//     setter: DateSetter,
//   },
//   {
//     type: 'options',
//     operations: selectOperations,
//     setter: SelectSetter,
//   },
// ];
