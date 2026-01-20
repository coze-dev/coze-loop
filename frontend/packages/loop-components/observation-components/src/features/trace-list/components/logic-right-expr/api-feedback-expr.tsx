// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import {
  Select,
  type SelectProps,
  CozInputNumber,
} from '@coze-arch/coze-design';

import { type Left } from '@/shared/components/analytics-logic-expr/logic-expr';
import { FeedbackApiType } from '@/shared/components/analytics-logic-expr/const';
import { useLocale } from '@/i18n';

export interface ApiFeedbackExprProps extends SelectProps {
  className?: string;
  left: Left;
}

const numberInputFormatter = (v: string | number) =>
  !Number.isNaN(parseFloat(`${v}`)) ? parseFloat(`${v}`).toString() : '';

export const ApiFeedbackExpr = (props: ApiFeedbackExprProps) => {
  const { t } = useLocale();
  const { left, className, disabled, value } = props;

  const feedbackType =
    left?.extra_info?.feedback_api_type ?? left?.extraInfo?.feedback_api_type;

  if (!feedbackType) {
    return null;
  }

  if (
    feedbackType === FeedbackApiType.Boolean ||
    feedbackType === FeedbackApiType.Category
  ) {
    const optionList =
      feedbackType === FeedbackApiType.Boolean
        ? [
            {
              label: t('yes'),
              value: 'true',
            },
            {
              label: t('no'),
              value: 'false',
            },
          ]
        : [];
    return (
      <Select
        key={feedbackType}
        value={value as string[]}
        onChange={v => props.onChange?.(v)}
        disabled={disabled}
        maxTagCount={2}
        showRestTagsPopover
        className={cls('max-w-full w-full overflow-hidden', className)}
        {...props}
        multiple={true}
        allowCreate={feedbackType === FeedbackApiType.Category}
        filter={true}
        optionList={optionList}
      />
    );
  }

  return (
    <CozInputNumber
      formatter={numberInputFormatter}
      max={Number.MAX_SAFE_INTEGER}
      min={Number.MIN_SAFE_INTEGER}
      hideButtons
      className={cls('w-full max-w-full overflow-hidden', className)}
      value={value?.[0]}
      onChange={v => {
        props.onChange?.(v);
      }}
      disabled={disabled}
    />
  );
};
