// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Expr, type RightRenderProps } from '../logic-expr';
import { findFieldByPath } from './utils';
import {
  useDataTypeList,
  type LogicFilterLeft,
  type RenderProps,
} from './logic-types';

export default function RightRender(
  props: RightRenderProps<
    LogicFilterLeft,
    string,
    string | number | undefined
  > &
    RenderProps,
) {
  const { expr, onExprChange, fields, disabled = false } = props;
  const field = findFieldByPath(fields, expr.left);
  if (!field) {
    return null;
  }
  if (expr.operator === 'is-empty' || expr.operator === 'is-not-empty') {
    return null;
  }

  const dataTypeList = useDataTypeList();
  const Setter =
    field?.setter ||
    dataTypeList.find(dataType => dataType.type === field.type)?.setter;

  if (!Setter) {
    return null;
  }

  return (
    <div className="w-48 grow overflow-hidden">
      <Setter
        {...(field.setterProps ?? {})}
        expr={expr as Expr<string, string, string>}
        field={field}
        disabled={disabled}
        value={expr.right as string}
        onChange={val => {
          onExprChange?.({
            ...expr,
            right: val as string | undefined,
          });
        }}
      />
    </div>
  );
}
