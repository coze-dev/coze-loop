import { QueryType } from '@cozeloop/api-schema/observation';
import { type OptionProps } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';

import { checkValueIsEmpty } from './right-render';
import { type Left } from './logic-expr';

interface ErrorMsgRenderProps {
  expr: {
    left?: Left;
    right?: string | number | string[] | number[];
    operator?: string | number | undefined;
  };
  tagLeftOption: OptionProps[];
  checkIsInvalidateExpr: (expr: Left | undefined) => boolean;
  valueChangeMap: Record<string, boolean>;
}

export const ErrorMsgRender = ({
  expr,
  tagLeftOption,
  checkIsInvalidateExpr,
  valueChangeMap,
}: ErrorMsgRenderProps) => {
  const { left, right, operator } = expr;
  const { t } = useLocale();

  const isInvalidateExpr = checkIsInvalidateExpr(left as Left | undefined);
  const leftname = tagLeftOption.find(item => item.value === left)?.label;

  if (isInvalidateExpr) {
    return (
      <div className="text-[#D0292F] text-[12px] whitespace-nowrap mt-1">
        {leftname ?? left?.type ?? ''} {t('filter_item_conflict')}
      </div>
    );
  }

  if (
    checkValueIsEmpty(right) &&
    operator !== QueryType.NotExist &&
    operator !== QueryType.Exist &&
    left &&
    valueChangeMap[left.value ?? '']
  ) {
    return (
      <div className="text-[#D0292F] text-[12px] whitespace-nowrap mt-1">
        {t('not_allowed_to_be_empty')}
      </div>
    );
  }

  return null;
};
