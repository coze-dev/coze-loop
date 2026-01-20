// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type IntlMessageFormat from 'intl-messageformat';

export function stringifyVal(val: unknown) {
  switch (typeof val) {
    case 'number':
    case 'bigint':
      return `${val}`;
    case 'boolean':
      return val ? 'true' : 'false';
    case 'string':
    case 'symbol':
      return val.toString();
    case 'object':
      return val === null
        ? ''
        : val instanceof Date
          ? val.toISOString()
          : JSON.stringify(val);
    default:
      return '';
  }
}

export function fillMissingOptions(
  messageFormat: IntlMessageFormat,
  options: Record<string, unknown>,
) {
  const ast = messageFormat.getAst();
  if (!ast?.length) {
    return undefined;
  }

  const missing: Record<string, string> = {};
  for (const element of ast) {
    // TYPE.ARGUMENT = 1
    if (element.type !== 1) {
      continue;
    }

    if (element.value in options) {
      continue;
    }

    missing[element.value] = '';
  }

  return { ...options, ...missing };
}
