// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties, type ReactNode } from 'react';

export type LogicOperator = 'and' | 'or';

export interface Expr<L = string, O = string, R = string> {
  left: L;
  operator: O;
  right: R;
  not?: boolean;
  disableDeletion?: boolean;
}

export interface ExprGroup<L = string, O = string, R = string> {
  logicOperator?: LogicOperator;
  not?: boolean;
  disableDeletion?: boolean;
  exprs?: Expr<L, O, R>[];
  childExprGroups?: ExprGroup<L, O, R>[];
}

export interface OperatorOption {
  label: ReactNode | string;
  value: string | number;
}

interface BaseRenderProps<L, O, R> {
  expr: Expr<L, O, R>;
  readonly?: boolean;
  size?: 'small' | 'large' | 'default';
  onExprChange?: (expr: Partial<Expr<L, O, R>>) => void;
}

export interface LeftRenderProps<L, O, R> extends BaseRenderProps<L, O, R> {
  onChange?: (val: L) => void;
}

export interface RightRenderProps<L, O, R> extends BaseRenderProps<L, O, R> {
  onChange?: (val: R) => void;
}

export interface OperatorRenderProps<L, O, R> extends BaseRenderProps<L, O, R> {
  onChange?: (val: O) => void;
}

interface Props<L, O, R> {
  enableNot?: boolean;
  readonly?: boolean;
  leftRender?: (props: LeftRenderProps<L, O, R>) => ReactNode;
  operatorRender?:
    | ((props: OperatorRenderProps<L, O, R>) => ReactNode)
    | OperatorOption[];
  rightRender?: (props: RightRenderProps<L, O, R>) => ReactNode;
  errorMsgRender?: (props: RightRenderProps<L, O, R>) => ReactNode;
  // maxExpressions?: number;
  // maxExpressionGroups?: number;
  // maxExpressionsPerGroup?: number;
  elementSize?: 'small' | 'large' | 'default';
  className?: string;
  style?: CSSProperties;
}

export interface LogicExprProps<L, O, R> extends Props<L, O, R> {
  value?: ExprGroup<L, O, R>;
  defaultExpr: Expr<L, O, R>;
  allowLogicOperators?: LogicOperator[];
  maxNestingDepth?: number;
  exprGroupRenderContentItemsClassName?: string;
  logicToggleReadonly?: boolean;
  onChange?: (exprGroup?: ExprGroup<L, O, R>) => void;
  onDeleteExpr?: (key: L) => void;
}

export interface ExprGroupRenderProps<L, O, R> extends Props<L, O, R> {
  value?: ExprGroup<L, O, R>;
  singleTag?: boolean;
  path: string;
  allowLogicOperators?: LogicOperator[];
  maxNestingDepth?: number;
  exprGroupRenderContentItemsClassName?: string;
  logicToggleReadonly?: boolean;
  onExprChange?: (expr: Expr<L, O, R>, path: string, index: number) => void;
  onExprGroupChange?: (exprGroup: ExprGroup<L, O, R>, path: string) => void;
  onAddItem?: (path: string) => void;
  onAddGroup?: (path: string) => void;
  onDeleteExpr?: (path: string, index: number) => void;
  onDeleteExprGroup?: (path: string) => void;
}

export interface ExprRenderProps<L, O, R> extends Props<L, O, R> {
  value: Expr<L, O, R>;
  path: string;
  onChange?: (expr: Expr<L, O, R>, path: string) => void;
  onDelete?: (path: string) => void;
  renderTool?: () => React.JSX.Element;
}
