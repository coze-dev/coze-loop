// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import React, { type ReactText } from 'react';

import { isEmpty } from 'lodash-es';
import classNames from 'classnames';
import { type FieldMeta } from '@cozeloop/api-schema/observation';
import { tag } from '@cozeloop/api-schema/data';
import {
  Input,
  Select,
  CozInputNumber,
  type SelectProps,
} from '@coze-arch/coze-design';

import {
  getOptionsWithKind,
  getOptionCopywriting,
  getLabelUnit,
} from './utils';
import { type Left, type CustomRightRenderMap } from './logic-expr';
import {
  EMPTY_RENDER_CMP_OP_LIST,
  type FilterFields,
  NUMBER_RENDER_CMP_OP_LIST,
  SELECT_MULTIPLE_RENDER_CMP_OP_LIST,
  SELECT_RENDER_CMP_OP_LIST,
} from './consts';
import { AUTO_EVAL_FEEDBACK } from './const';

import styles from './index.module.less';

export const checkValueIsEmpty = (
  value?: number | number[] | string | string[] | null,
) =>
  isEmpty(value) ||
  (typeof value === 'string' && value.trim() === '') ||
  value === undefined ||
  value === null;

export interface RightRenderProps {
  left?: Left;
  operator?: number | string;
  right?: string | number | string[] | number[];
  disabled?: boolean;
  defaultImmutableKeys?: string[];
  isInvalidateExpr?: boolean;
  valueChanged?: boolean;
  tagFilterRecord: Record<string, FieldMeta>;
  onRightValueChange?: (value: string | number | string[] | number[]) => void;
  onValueChangeStatus?: (fieldName: string, changed: boolean) => void;
  customRightRenderMap?: CustomRightRenderMap;
}

export const RightRender: React.FC<RightRenderProps> = props => {
  const {
    left,
    operator,
    right,
    disabled,
    defaultImmutableKeys,
    isInvalidateExpr,
    valueChanged,
    tagFilterRecord,
    onRightValueChange,
    onValueChangeStatus,
    customRightRenderMap,
  } = props;

  const { field_options, value_type, support_customizable_option } =
    tagFilterRecord[left?.type ?? ''] || {};

  const options = getOptionsWithKind({
    fieldOptions: field_options,
    valueKind: value_type,
  });

  if (
    !left ||
    !operator ||
    EMPTY_RENDER_CMP_OP_LIST.includes(String(operator))
  ) {
    return <div className={styles['expr-value-item-content']} />;
  }

  const multipleSelectProps: Partial<SelectProps> = {
    allowCreate: support_customizable_option,
    filter: support_customizable_option,
    multiple: true,
    maxTagCount: 4,
    ellipsisTrigger: true,
    showRestTagsPopover: true,
    restTagsPopoverProps: {
      position: 'top',
      stopPropagation: true,
    },
  };

  const showSelect =
    SELECT_MULTIPLE_RENDER_CMP_OP_LIST.includes(String(operator)) ||
    SELECT_RENDER_CMP_OP_LIST.includes(String(operator)) ||
    left?.extraInfo?.content_type === tag.TagContentType.Boolean;

  const isMultiple = SELECT_MULTIPLE_RENDER_CMP_OP_LIST.includes(
    String(operator),
  );
  const fieldKey = left?.type ?? '';

  const isNumberInput =
    NUMBER_RENDER_CMP_OP_LIST.includes(fieldKey as FilterFields) ||
    left?.extraInfo?.content_type === tag.TagContentType.ContinuousNumber;

  const numberInputFormatter =
    left.type === AUTO_EVAL_FEEDBACK ||
    left?.extraInfo?.content_type === tag.TagContentType.ContinuousNumber
      ? (v: string | number) =>
          !Number.isNaN(parseFloat(`${v}`)) ? parseFloat(`${v}`).toString() : ''
      : (v: string | number) => `${v}`.replace(/\D/g, '');
  const customRightRender =
    customRightRenderMap?.[fieldKey] ??
    customRightRenderMap?.[left?.extraInfo?.content_type ?? ''];

  if (customRightRender) {
    return (
      <div
        className={classNames(styles['expr-value-item-content'], {
          [styles['expr-value-item-content-invalidate']]:
            isInvalidateExpr || (checkValueIsEmpty(right) && valueChanged),
        })}
      >
        {customRightRender?.({
          disabled: disabled || defaultImmutableKeys?.includes(fieldKey),
          style: { width: '100%' },
          value: right,
          onChange: v => {
            onRightValueChange?.(v as string[] | number[] | string | number);
            onValueChangeStatus?.(left?.value ?? '', true);
          },
          optionList: options?.map(item => ({
            label: getOptionCopywriting(left?.type ?? '', item),
            value: item,
          })),
          left,
          ...(isMultiple ? multipleSelectProps : {}),
        })}
      </div>
    );
  }

  return (
    <div
      className={classNames(styles['expr-value-item-content'], {
        [styles['expr-value-item-content-invalidate']]:
          isInvalidateExpr || (checkValueIsEmpty(right) && valueChanged),
      })}
    >
      {operator && showSelect ? (
        <Select
          dropdownClassName={styles['render-select']}
          disabled={
            disabled || defaultImmutableKeys?.includes(left?.type ?? '')
          }
          style={{ width: '100%' }}
          value={right}
          onChange={v => {
            onRightValueChange?.(v as string[] | number[] | string | number);
            onValueChangeStatus?.(fieldKey, true);
          }}
          optionList={options?.map(item => ({
            label: getOptionCopywriting(fieldKey, item),
            value: item,
          }))}
          {...(isMultiple ? multipleSelectProps : {})}
        />
      ) : isNumberInput ? (
        <CozInputNumber
          formatter={numberInputFormatter}
          disabled={disabled}
          hideButtons
          value={right?.[0] as string}
          max={Number.MAX_SAFE_INTEGER}
          min={Number.MIN_SAFE_INTEGER}
          onChange={v => {
            onRightValueChange?.(numberInputFormatter(`${v}`) as string);
            onValueChangeStatus?.(fieldKey, true);
          }}
          suffix={getLabelUnit(fieldKey)}
        />
      ) : (
        <Input
          disabled={disabled}
          value={right as ReactText}
          onChange={v => {
            onRightValueChange?.(v);
            onValueChangeStatus?.(fieldKey, true);
          }}
        />
      )}
    </div>
  );
};
