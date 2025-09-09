// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */

import { isNumber } from 'lodash-es';
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { IconCozPlus, IconCozPlusCircle } from '@coze-arch/coze-design/icons';
import { Button, Divider, type ButtonProps } from '@coze-arch/coze-design';

import type { ExprGroup, ExprGroupRenderProps } from './types';
import { LogicToggle } from './logic-toggle';
import { ExprRender } from './expr-render';

import styles from './index.module.less';

const ToolButton = (props: ButtonProps) => (
  <Button
    className={styles['expr-render-tool-btn']}
    theme="borderless"
    type="primary"
    size="small"
    {...props}
  >
    {props.children}
  </Button>
);

function genPath(path: string, index: number) {
  return path ? `${path}-${index}` : `${index}`;
}

export function ExprGroupRender<L, O, R>({
  value,
  readonly,
  singleTag,
  enableNot,
  className,
  exprGroupRenderContentItemsClassName,
  style,
  path,
  leftRender,
  operatorRender,
  rightRender,
  allowLogicOperators,
  maxNestingDepth,
  elementSize = 'default',
  logicToggleReadonly,
  onExprChange,
  onExprGroupChange,
  onAddItem,
  onAddGroup,
  onDeleteExpr,
  onDeleteExprGroup,
  errorMsgRender,
}: ExprGroupRenderProps<L, O, R>) {
  const isEdit = !readonly;

  const patchExpr = (expr: Partial<ExprGroup<L, O, R>>) => {
    if (!value) {
      return;
    }

    const newExpr = { ...value, ...expr };

    onExprGroupChange?.(newExpr, path);
  };

  const renderTool = () => {
    if (!isEdit) {
      return null;
    }

    const currentNestingDepth = path ? path.split('-').length + 1 : 1;

    const showAddGroupButton = isNumber(maxNestingDepth)
      ? currentNestingDepth < (maxNestingDepth as number)
      : true;

    return (
      <div
        className={styles['expr-render-tool']}
        style={value ? undefined : { marginTop: 0 }}
      >
        <ToolButton
          className="add-filter-btn"
          icon={
            <span className="text-brand-9 flex items-center justify-center text-[14px]">
              <IconCozPlus className="w-[14px] h-[14px]" />
            </span>
          }
          onClick={() => onAddItem?.(path)}
          color="secondary"
          type="primary"
        >
          <span className="text-brand-9 text-[14px] leading-[20px]">
            {I18n.t('logic_expr_add_filter')}
          </span>
        </ToolButton>
        {showAddGroupButton ? (
          <>
            <Divider layout="vertical" />
            <ToolButton
              icon={<IconCozPlusCircle />}
              onClick={() => onAddGroup?.(path)}
            >
              {I18n.t('logic_expr_add_filter_group')}
            </ToolButton>
          </>
        ) : null}
      </div>
    );
  };

  if (!value) {
    return isEdit ? (
      renderTool()
    ) : (
      <div className={styles['expr-render-text']}>-</div>
    );
  }

  const exprItemsNumber =
    (value.exprs?.length || 0) + (value.childExprGroups?.length || 0);

  if (
    !path &&
    value.exprs &&
    value.exprs.length === 1 &&
    singleTag &&
    (!value.childExprGroups || value.childExprGroups.length === 0)
  ) {
    return (
      <>
        <ExprRender
          path={path}
          value={value.exprs[0]}
          readonly={readonly}
          enableNot={enableNot}
          leftRender={leftRender}
          rightRender={rightRender}
          errorMsgRender={errorMsgRender}
          operatorRender={operatorRender}
          elementSize={elementSize}
          onChange={(itemExpr, itemPath) => {
            onExprChange?.(itemExpr, itemPath, 0);
          }}
          onDelete={itemPath => {
            onDeleteExpr?.(itemPath, 0);
          }}
        />
        {renderTool()}
      </>
    );
  }

  return (
    <div
      className={classNames(
        styles['expr-render-group'],
        // {
        //   [styles['expr-render-group_delete']]: deleteButtonHover,
        // },
        className,
      )}
      style={style}
    >
      <div className={styles['expr-render-group-content']}>
        {exprItemsNumber >= 1 ? (
          <LogicToggle
            operator={value.logicOperator}
            enableNot={enableNot}
            not={value.not}
            readonly={readonly || logicToggleReadonly}
            hideLine={exprItemsNumber === 1}
            allowLogicOperators={allowLogicOperators}
            onChange={(v1, v2) => patchExpr({ logicOperator: v1, not: v2 })}
          />
        ) : null}
        <div
          className={classNames(
            styles['expr-render-expr-items'],
            exprGroupRenderContentItemsClassName,
          )}
        >
          {value?.exprs?.map((expr, index) => (
            <ExprRender
              path={path}
              key={index}
              value={expr}
              readonly={readonly}
              enableNot={enableNot}
              leftRender={leftRender}
              rightRender={rightRender}
              errorMsgRender={errorMsgRender}
              operatorRender={operatorRender}
              elementSize={elementSize}
              onChange={(itemExpr, itemPath) => {
                onExprChange?.(itemExpr, itemPath, index);
              }}
              onDelete={itemPath => {
                onDeleteExpr?.(itemPath, index);
              }}
            />
          ))}
          {value?.childExprGroups?.map((subExprGroup, index) => (
            <ExprGroupRender
              path={genPath(path, index)}
              key={index}
              value={subExprGroup}
              readonly={readonly}
              enableNot={enableNot}
              leftRender={leftRender}
              rightRender={rightRender}
              errorMsgRender={errorMsgRender}
              operatorRender={operatorRender}
              allowLogicOperators={allowLogicOperators}
              maxNestingDepth={maxNestingDepth}
              elementSize={elementSize}
              onExprChange={onExprChange}
              onExprGroupChange={onExprGroupChange}
              onAddItem={onAddItem}
              onAddGroup={onAddGroup}
              onDeleteExpr={onDeleteExpr}
              onDeleteExprGroup={onDeleteExprGroup}
              className={className}
              style={style}
              exprGroupRenderContentItemsClassName={
                exprGroupRenderContentItemsClassName
              }
            />
          ))}
        </div>
      </div>
      {renderTool()}
    </div>
  );
}
