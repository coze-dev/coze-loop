// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import classNames from 'classnames';
import { TagSelect } from '@cozeloop/tag-components';
import { type LeftRenderProps } from '@cozeloop/components';
import { type OptionProps, Select } from '@coze-arch/coze-design';

import { type Left } from '../logic-expr/logic-expr';
import styles from '../logic-expr/index.module.less';
import { MANUAL_FEEDBACK, MANUAL_FEEDBACK_PREFIX } from '../logic-expr/const';

export interface LeftManualExprProps
  extends LeftRenderProps<
    Left,
    number | undefined,
    string | number | string[] | number[] | undefined
  > {
  disabled?: boolean;
  defaultImmutableKeys?: string[];
  tagLeftOption: OptionProps[];
  isInvalidateExpr: boolean;
  onLeftExprTypeChange?: (type: string, left: Left) => void;
  onLeftExprValueChange?: (value: string, left: Left) => void;
}

export const LeftManualExpr = (props: LeftManualExprProps) => {
  const {
    expr,
    onExprChange,
    disabled,
    defaultImmutableKeys,
    tagLeftOption,
    isInvalidateExpr,
    onLeftExprTypeChange,
    onLeftExprValueChange,
  } = props;

  const { left } = expr;

  return (
    <div
      className={classNames(
        styles['expr-value-item-content'],
        'flex items-center gap-2 !min-w-[280px]',
        {
          [styles['expr-value-item-content-invalidate']]: isInvalidateExpr,
        },
      )}
    >
      <Select
        dropdownClassName={classNames(styles['render-select'], 'flex-1')}
        filter
        style={{ width: '100%', fontSize: '13px' }}
        defaultOpen={!left?.type}
        disabled={disabled || defaultImmutableKeys?.includes(left?.value ?? '')}
        value={left?.type}
        onChange={v => {
          const typedValue = v as string;
          onLeftExprTypeChange?.(typedValue, left);
          onExprChange?.({
            left: {
              type: typedValue,
              value:
                typedValue === MANUAL_FEEDBACK
                  ? (left?.value ?? '')
                  : typedValue,
            },
            operator: undefined,
            right: undefined,
          });
        }}
        optionList={tagLeftOption}
      />
      <TagSelect
        dropdownClassName={classNames(styles['render-select'], 'flex-1')}
        filter
        style={{ width: '100%', fontSize: '13px' }}
        defaultOpen={!left?.value}
        value={(left?.value ?? '').slice(MANUAL_FEEDBACK_PREFIX.length)}
        onChange={v => {
          const { value, label, ...rest } = v as any;
          const { content_type, tag_key_id } = rest;

          const typedValue = value as string;
          onLeftExprValueChange?.(typedValue, left);
          onExprChange?.({
            left: {
              type: left?.type,
              value: `${MANUAL_FEEDBACK_PREFIX}${typedValue}`,
              extraInfo: {
                content_type,
                tag_key_id,
              },
            },
            operator: undefined,
            right: undefined,
          });
        }}
        onChangeWithObject
        showDisableTag
      />
    </div>
  );
};
