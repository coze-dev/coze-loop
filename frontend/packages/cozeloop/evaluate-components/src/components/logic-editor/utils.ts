// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type LogicField } from './logic-types';

export function findFieldByPath(
  fields: LogicField[],
  fieldPaths: string | string[],
): LogicField | undefined {
  if (!fieldPaths) {
    return undefined;
  }
  const fieldPath = Array.isArray(fieldPaths)
    ? fieldPaths
    : fieldPaths
      ? [fieldPaths]
      : [];
  let field: LogicField | undefined;
  let targetFields: LogicField[] = fields;
  fieldPath?.forEach(fieldName => {
    field = targetFields.find(item => item.name === fieldName);
    targetFields = field?.children ?? [];
  });
  return field;
}
