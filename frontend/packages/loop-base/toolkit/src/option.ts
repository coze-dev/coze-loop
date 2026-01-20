// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/naming-convention */
import { type ReactNode } from 'react';

export interface OptionItem<T> {
  label: ReactNode;
  value: T;
}

/**
 * 将 Record<value, label> 转换成 OptionItem 数组
 * @param record Record<value, label>
 * @param {boolean} autoParseNumber 是否需要将可 parse 为 number 的 key parse 成 number
 *    由于 Object.entries 无法判断 key 是否为 number，因此需要将 number 类型的 key parse 成 number
 *    了解到服务端 enum 只能定义为 number，因此这里默认为 true
 *
 * @example
 * const record = {1: 'a', 2: 'b'};
 * fromRecord(record);
 * // [{label: 'a', value: 1}, {label: 'b', value: 2}]
 */
function fromRecord<T extends string | number>(
  record: Partial<Record<T, ReactNode>>,
  autoParseNumber = true,
): Array<OptionItem<T>> {
  return Object.entries<ReactNode>(record).map(([value, label]) => ({
    label,
    value: (autoParseNumber
      ? isNaN(Number(value))
        ? value
        : Number(value)
      : value) as T,
  }));
}

export const OptionUtils = {
  fromRecord,
};
