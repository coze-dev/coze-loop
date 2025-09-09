// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozCross } from '@coze-arch/coze-design/icons';
import { IconButton, Input, Select, Tooltip } from '@coze-arch/coze-design';

import type {
  Expr,
  ExprRenderProps,
  LeftRenderProps,
  OperatorRenderProps,
  RightRenderProps,
} from './types';
import { LogicNot } from './logic-not';

import styles from './index.module.less';

function defaultLeftRender<L, O, R>({
  expr,
  readonly,
  size,
  onChange,
  onExprChange,
}: LeftRenderProps<L, O, R>) {
  const onInputChange = (val: L) => {
    onExprChange ? onExprChange({ ...expr, left: val }) : onChange?.(val);
  };

  return !readonly ? (
    <Input
      size={size}
      value={`${expr.left}`}
      onChange={value => {
        onInputChange(value as L);
      }}
    />
  ) : (
    <div className={styles['expr-render-text']}>{expr.left as string}</div>
  );
}

function defaultOperatorRender<L, O, R>({
  expr,
  readonly,
  size,
  onChange,
  onExprChange,
}: OperatorRenderProps<L, O, R>) {
  const onInputChange = (val: O) => {
    onExprChange ? onExprChange({ ...expr, operator: val }) : onChange?.(val);
  };

  return !readonly ? (
    <Input
      size={size}
      value={`${expr.operator}`}
      onChange={value => {
        onInputChange(value as O);
      }}
    />
  ) : (
    <div className={styles['expr-render-text']} style={{ margin: '0 8px' }}>
      {expr.operator as string}
    </div>
  );
}

function defaultRightRender<L, O, R>({
  expr,
  readonly,
  size,
  onChange,
  onExprChange,
}: RightRenderProps<L, O, R>) {
  const onInputChange = (val: R) => {
    onExprChange ? onExprChange({ ...expr, right: val }) : onChange?.(val);
  };

  return !readonly ? (
    <Input
      size={size}
      value={`${expr.right}`}
      onChange={value => {
        onInputChange(value as R);
      }}
    />
  ) : (
    <div className={styles['expr-render-text']}>{expr.right as string}</div>
  );
}

export const ExprRender = <L, O, R>({
  value,
  readonly,
  enableNot,
  className,
  style,
  path,
  leftRender,
  operatorRender,
  rightRender,
  elementSize = 'default',
  onChange,
  onDelete,
  renderTool,
  errorMsgRender,
}: ExprRenderProps<L, O, R>) => {
  const isEdit = !readonly;

  const [deleteButtonHover, setDeleteButtonHover] = useState(false);

  const patchExpr = (expr: Partial<Expr<L, O, R>>) => {
    const newExpr = { ...value, ...expr };

    onChange?.(newExpr, path);
  };

  const renderLeft = () =>
    (leftRender || defaultLeftRender)({
      expr: value,
      readonly,
      size: elementSize,
      onChange: left => patchExpr({ left }),
      onExprChange: patchExpr,
    });

  const renderRight = () =>
    (rightRender || defaultRightRender)({
      expr: value,
      readonly,
      size: elementSize,
      onChange: right => patchExpr({ right }),
      onExprChange: patchExpr,
    });

  const renderOperator = () => {
    if (Array.isArray(operatorRender)) {
      return (
        <Select
          style={{ width: '100%', minWidth: 50 }}
          value={`${value.operator}`}
          disabled={readonly}
          onChange={v => patchExpr({ operator: v as O })}
          optionList={operatorRender}
        />
      );
    }

    return (operatorRender || defaultOperatorRender)({
      expr: value,
      readonly,
      size: elementSize,
      onChange: operator => patchExpr({ operator }),
      onExprChange: patchExpr,
    });
  };

  const renderErrorMsg = () => {
    if (errorMsgRender) {
      return errorMsgRender({
        expr: value,
      });
    }
  };
  return (
    <>
      <div
        className={classNames(
          styles['expr-render-expr-item'],
          {
            [styles['expr-render-expr-item_delete']]: deleteButtonHover,
          },
          className,
        )}
        style={style}
      >
        {enableNot ? (
          <LogicNot
            not={value.not}
            readonly={readonly}
            onChange={not => patchExpr({ not })}
          />
        ) : null}
        <div className="flex gap-x-2 w-full flex-col">
          <div className="flex gap-x-2 w-full items-center">
            {renderLeft()}
            {renderOperator()}
            {renderRight()}
            {isEdit ? (
              <Tooltip
                theme="dark"
                content={I18n.t('logic_expr_delete_filter')}
              >
                <IconButton
                  className={classNames(
                    'expr-render-del-btn',
                    styles['expr-render-del-btn'],
                  )}
                  size="small"
                  color="secondary"
                  disabled={value.disableDeletion}
                  icon={<IconCozCross />}
                  onClick={() => {
                    setDeleteButtonHover(false);
                    onDelete?.(path);
                  }}
                  onMouseEnter={() => {
                    setDeleteButtonHover(true);
                  }}
                  onMouseLeave={() => {
                    setDeleteButtonHover(false);
                  }}
                />
              </Tooltip>
            ) : null}
          </div>
          {renderErrorMsg()}
        </div>
      </div>
      {path ? null : renderTool?.()}
    </>
  );
};
