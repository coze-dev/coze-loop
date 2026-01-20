// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useState } from 'react';

import { cloneDeep, get, isNumber, set } from 'lodash-es';

import type { Expr, ExprGroup, LogicExprProps } from './types';
import { ExprGroupRender } from './expr-group-render';

function getObjectPath(path?: string, index?: number) {
  let groupPath = path
    ? path
        .split('-')
        .map(p => `childExprGroups[${p}]`)
        .join('.')
    : '';
  if (isNumber(index)) {
    groupPath = `${groupPath}${groupPath ? '.' : ''}exprs[${index}]`;
  }
  return groupPath;
}

function splitPath(path: string): [string, string] {
  const last = path.lastIndexOf('-');
  if (last === -1) {
    return ['', path];
  }
  return [path.substring(0, last), path.substring(last + 1)];
}

// eslint-disable-next-line @coze-arch/max-line-per-function
export function LogicExpr<L = string, O = string, R = string>({
  defaultExpr,
  value,
  onChange: _onChange,
  ...restProps
}: LogicExprProps<L, O, R>) {
  const [singleTag, setSingleTag] = useState(false);

  // 通用特化逻辑处理
  const onChange = (exprGroup?: ExprGroup<L, O, R>) => {
    let processedExprGroup = exprGroup;
    if (
      (!exprGroup?.childExprGroups || exprGroup.childExprGroups.length === 0) &&
      exprGroup?.exprs?.length === 1
    ) {
      processedExprGroup = {
        exprs: exprGroup.exprs,
      };
    }
    _onChange?.(processedExprGroup);
  };

  const onExprGroupChange = (exprGroup: ExprGroup<L, O, R>, path: string) => {
    const objectPath = getObjectPath(path);
    if (!value || !objectPath) {
      onChange(exprGroup);
      return;
    }
    const newValue = set(cloneDeep(value), objectPath, exprGroup);

    onChange(newValue);
  };

  const onExprChange = (expr: Expr<L, O, R>, path: string, index: number) => {
    if (!value) {
      setSingleTag(true);
      onChange({
        exprs: [expr],
      });
      return;
    }
    const objectPath = getObjectPath(path, index);
    const newValue = set(cloneDeep(value), objectPath, expr);

    onChange(newValue);
  };

  const onAddExprItem = (path: string) => {
    const clonedDefaultExpr = cloneDeep(defaultExpr);
    if (!value) {
      onExprChange(clonedDefaultExpr, path, 0);
      return;
    }

    const clonedGroupValue = cloneDeep(value);
    const targetGroupValue: ExprGroup<L, O, R> = path
      ? get(clonedGroupValue, getObjectPath(path))
      : clonedGroupValue;

    if (targetGroupValue.exprs?.length) {
      targetGroupValue.exprs.push(clonedDefaultExpr);
    } else {
      targetGroupValue.exprs = [clonedDefaultExpr];
    }

    onExprGroupChange(targetGroupValue, path);
  };

  const onAddExprGroup = (path: string) => {
    const clonedDefaultExpr = cloneDeep(defaultExpr);
    const emptyGroup: ExprGroup<L, O, R> = {
      exprs: [clonedDefaultExpr],
    };
    if (!value) {
      setSingleTag(false);
      onExprGroupChange(emptyGroup, path);
      return;
    }

    const clonedGroupValue = cloneDeep(value);
    const targetGroupValue: ExprGroup<L, O, R> = path
      ? get(clonedGroupValue, getObjectPath(path))
      : clonedGroupValue;

    if (targetGroupValue.childExprGroups?.length) {
      targetGroupValue.childExprGroups.push(emptyGroup);
    } else {
      targetGroupValue.childExprGroups = [emptyGroup];
    }

    onExprGroupChange(targetGroupValue, path);
  };

  const findDelPath = (path: string): string => {
    const [parentPath] = splitPath(path);

    if (!parentPath) {
      return path;
    }

    const parentGroup: ExprGroup<L, O, R> = get(
      value,
      getObjectPath(parentPath),
    );

    const needDeleteParent =
      (parentGroup.childExprGroups?.length || 0) +
        (parentGroup.exprs?.length || 0) <=
      1;

    if (needDeleteParent) {
      return findDelPath(parentPath);
    }

    return path;
  };

  const onDeleteExprGroup = (path: string) => {
    if (!value) {
      return;
    }

    const clonedValue = cloneDeep(value);

    const baseDelPath = findDelPath(path);

    const [parentPath, selfPath] = splitPath(baseDelPath);

    if (!baseDelPath) {
      onChange(undefined);
      return;
    }

    if (!parentPath) {
      const needDeleteRoot =
        (clonedValue.childExprGroups?.length || 0) +
          (clonedValue.exprs?.length || 0) <=
        1;

      if (needDeleteRoot) {
        onChange(undefined);
        return;
      }
    }

    const parentGroup: ExprGroup<L, O, R> = parentPath
      ? get(clonedValue, getObjectPath(parentPath))
      : clonedValue;

    const selfPathIndex = parseInt(selfPath, 10);

    parentGroup.childExprGroups?.splice(selfPathIndex, 1);

    if (
      parentGroup.childExprGroups?.length &&
      parentGroup.childExprGroups.length === 1 &&
      (!parentGroup.exprs || parentGroup.exprs.length === 0)
    ) {
      if (parentPath) {
        const newValue = set(
          clonedValue,
          getObjectPath(parentPath),
          parentGroup.childExprGroups[0],
        );
        onChange(newValue);
      } else {
        onChange(parentGroup.childExprGroups[0]);
      }
      return;
    }

    onChange(clonedValue);
  };

  const onDeleteExpr = (path: string, index: number) => {
    if (!value) {
      return;
    }

    const clonedValue = cloneDeep(value);

    const parentGroup: ExprGroup<L, O, R> = path
      ? get(clonedValue, getObjectPath(path))
      : clonedValue;

    const needDeleteParent =
      (parentGroup.childExprGroups?.length || 0) +
        (parentGroup.exprs?.length || 0) <=
      1;

    if (needDeleteParent) {
      onDeleteExprGroup(path);
      restProps.onDeleteExpr?.(parentGroup?.exprs?.[index]?.left as L);
    } else {
      parentGroup?.exprs?.splice(index, 1);
      if (
        parentGroup.childExprGroups?.length &&
        parentGroup.childExprGroups.length === 1 &&
        (!parentGroup.exprs || parentGroup.exprs.length === 0)
      ) {
        if (path) {
          const newValue = set(
            clonedValue,
            getObjectPath(path),
            parentGroup.childExprGroups[0],
          );
          onChange(newValue);
        } else {
          onChange(parentGroup.childExprGroups[0]);
        }
        return;
      }
      onChange(clonedValue);
    }
  };

  return (
    <ExprGroupRender<L, O, R>
      path=""
      singleTag={singleTag}
      value={value}
      {...restProps}
      onExprChange={onExprChange}
      onExprGroupChange={onExprGroupChange}
      onAddItem={onAddExprItem}
      onAddGroup={onAddExprGroup}
      onDeleteExprGroup={onDeleteExprGroup}
      onDeleteExpr={onDeleteExpr}
    />
  );
}
