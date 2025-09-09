// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isFunction } from 'lodash-es';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Obj = Record<string, any>;

// region array2Map 重载声明
// 和 OptionUtil.array2Map 虽然相似，但在用法和类型约束上还是很不一样的
/**
 * 将列表转化为 map
 * @param items
 * @param key 指定 item[key] 作为 map 的键
 * @example
 * const items = [{name: 'a', id: 1}];
 * array2Map(items, 'id');
 * // {1: {name: 'a', id: 1}}
 */
function array2Map<T extends Obj, K extends keyof T>(
  items: T[],
  key: K,
): Record<T[K], T>;
/**
 * 将列表转化为 map
 * @param items
 * @param key 指定 item[key] 作为 map 的键
 * @param value 指定 item[value] 作为 map 的值
 * @example
 * const items = [{name: 'a', id: 1}];
 * array2Map(items, 'id', 'name');
 * // {1: 'a'}
 */
function array2Map<T extends Obj, K extends keyof T, V extends keyof T>(
  items: T[],
  key: K,
  value: V,
): Record<T[K], T[V]>;
/**
 * 将列表转化为 map
 * @param items
 * @param key 指定 item[key] 作为 map 的键
 * @param value 获取值
 * @example
 * const items = [{name: 'a', id: 1}];
 * array2Map(items, 'id', (item) => `${item.id}-${item.name}`);
 * // {1: '1-a'}
 */
function array2Map<T extends Obj, K extends keyof T, V>(
  items: T[],
  key: K,
  value: (item: T) => V,
): Record<T[K], V>;
// endregion
/* eslint-disable @typescript-eslint/no-explicit-any */
/** 将列表转化为 map */
function array2Map<T extends Obj, K extends keyof T>(
  items: T[],
  key: K,
  value: keyof T | ((item: T) => any) = item => item,
): Partial<Record<T[K], any>> {
  return items.reduce((map, item) => {
    const currKey = String(item[key]);
    const currValue = isFunction(value) ? value(item) : item[value];
    return { ...map, [currKey]: currValue };
  }, {});
}

// eslint-disable-next-line @typescript-eslint/naming-convention
export const ArrayUtils = {
  array2Map,
};
