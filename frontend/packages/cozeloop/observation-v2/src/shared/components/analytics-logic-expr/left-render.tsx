/* eslint-disable complexity */
import { useMemo } from 'react';

import cls from 'classnames';
import { Select, type OptionProps } from '@coze-arch/coze-design';

import { type Left, type CustomRightRenderMap } from './logic-expr';

import styles from './index.module.less';

interface LeftRendererProps {
  expr: {
    left?: Left;
  };
  onExprChange?: (value: {
    left?: Left;
    operator?: number;
    right?: string | number | string[] | number[];
  }) => void;
  tagLeftOption: OptionProps[];
  disabled?: boolean;
  defaultImmutableKeys?: string[];
  disabledRowKeys?: string[];
  checkIsInvalidateExpr: (expr: Left | undefined) => boolean;
  customLeftRenderMap: CustomRightRenderMap;
}

export const LeftRenderer = (props: LeftRendererProps) => {
  const {
    expr,
    onExprChange,
    tagLeftOption,
    disabled,
    defaultImmutableKeys,
    disabledRowKeys,
    checkIsInvalidateExpr,
    customLeftRenderMap,
  } = props;
  const { left } = expr;
  const isInvalidateExpr = checkIsInvalidateExpr(left);
  // 检查当前行是否应该被禁用
  const isRowDisabled = disabledRowKeys?.includes(left?.type ?? '') || false;
  const CustomLeftRender = useMemo(
    () => customLeftRenderMap[left?.type ?? ''],
    [left?.type],
  );

  if (CustomLeftRender) {
    return <CustomLeftRender {...props} />;
  }

  return (
    <div
      className={cls(styles['expr-value-item-content'], {
        [styles['expr-value-item-content-invalidate']]: isInvalidateExpr,
      })}
    >
      <Select
        dropdownClassName={styles['render-select']}
        filter
        style={{ width: '100%' }}
        defaultOpen={!left?.type && !disabled}
        disabled={
          disabled ||
          isRowDisabled ||
          defaultImmutableKeys?.includes(left?.type ?? '')
        }
        value={left?.type ?? ''}
        onChange={v => {
          const typedValue = v as string;
          onExprChange?.({
            left: { type: typedValue, value: undefined },
            operator: undefined,
            right: undefined,
          });
        }}
        optionList={tagLeftOption}
      />
    </div>
  );
};
