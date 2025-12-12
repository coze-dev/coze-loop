/* eslint-disable @typescript-eslint/no-explicit-any */
import cls from 'classnames';
import { type OptionProps, Select } from '@coze-arch/coze-design';

import { type LeftRenderProps } from '@/shared/components/logic-expr';
import { type Left } from '@/shared/components/analytics-logic-expr/logic-expr';
import styles from '@/shared/components/analytics-logic-expr/index.module.less';
import {
  MANUAL_FEEDBACK,
  MANUAL_FEEDBACK_PREFIX,
} from '@/shared/components/analytics-logic-expr/const';

import { LabelSelect } from '../label-select';

export interface ManualFeedbackExprProps
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
  customParams: Record<string, any>;
}

export const ManualFeedbackExpr = (props: ManualFeedbackExprProps) => {
  const {
    expr,
    onExprChange,
    disabled,
    defaultImmutableKeys,
    tagLeftOption,
    isInvalidateExpr,
    onLeftExprTypeChange,
    onLeftExprValueChange,
    customParams,
  } = props;

  const { left } = expr;

  return (
    <div
      className={cls(
        styles['expr-value-item-content'],
        'flex items-center gap-2 !min-w-[280px]',
        {
          [styles['expr-value-item-content-invalidate']]: isInvalidateExpr,
        },
      )}
    >
      <Select
        dropdownClassName={cls(styles['render-select'], 'flex-1')}
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
      <LabelSelect
        disabled={disabled}
        customParams={customParams}
        dropdownClassName={cls(styles['render-select'], 'flex-1')}
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
              extra_info: {
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
