// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { Input, type OptionProps, Select } from '@coze-arch/coze-design';

import { type LeftRenderProps } from '@/shared/components/logic-expr';
import { type Left } from '@/shared/components/analytics-logic-expr/logic-expr';
import styles from '@/shared/components/analytics-logic-expr/index.module.less';
import {
  API_FEEDBACK,
  API_FEEDBACK_OPERATORS,
  API_FEEDBACK_PREFIX,
  FeedbackApiType,
} from '@/shared/components/analytics-logic-expr/const';
import { useLocale } from '@/i18n';

export interface ApiFeedbackExprProps
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

export const ApiFeedbackExpr = (props: ApiFeedbackExprProps) => {
  const { t } = useLocale();
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

  const { left, operator, right } = expr;

  return (
    <div
      className={cls(
        styles['expr-value-item-content'],
        'flex items-center gap-2 !min-w-[380px]',
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
                typedValue === API_FEEDBACK ? (left?.value ?? '') : typedValue,
              extra_info: left?.extra_info ?? left?.extraInfo ?? {},
            },
            operator: undefined,
            right: undefined,
          });
        }}
        optionList={tagLeftOption}
      />
      <Input
        disabled={disabled}
        style={{ width: '100%', fontSize: '13px' }}
        value={(left?.value ?? '').slice(API_FEEDBACK_PREFIX.length)}
        onChange={v => {
          const typedValue = v as string;
          onLeftExprValueChange?.(typedValue, left);
          onExprChange?.({
            left: {
              type: left?.type,
              value: `${API_FEEDBACK_PREFIX}${typedValue}`,
              extra_info: left?.extra_info ?? left?.extraInfo ?? {},
            },
            operator,
            right,
          });
        }}
      />
      <Select
        disabled={disabled}
        style={{ width: '100%', fontSize: '13px', maxWidth: '100px' }}
        placeholder={t('select_label_type')}
        optionList={[
          { label: t('category_label'), value: FeedbackApiType.Category },
          { label: t('numeric_label'), value: FeedbackApiType.Number },
          { label: t('boolean_label'), value: FeedbackApiType.Boolean },
        ]}
        value={
          left?.extra_info?.feedback_api_type ??
          left?.extraInfo?.feedback_api_type
        }
        onChange={v => {
          const typedValue = v as string;
          onLeftExprValueChange?.(typedValue, left);
          const op = API_FEEDBACK_OPERATORS[typedValue][0];
          onExprChange?.({
            left: {
              type: left?.type,
              value: left?.value,
              extra_info: {
                feedback_api_type: typedValue,
              },
            },
            operator: op,
            right: undefined,
          });
        }}
      />
    </div>
  );
};
