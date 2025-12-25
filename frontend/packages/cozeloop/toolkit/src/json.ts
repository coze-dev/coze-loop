// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @coze-arch/use-error-in-catch */
import JSONBig from 'json-bigint';

const jsonBigIntToString = JSONBig({ storeAsString: true });
export const safeJsonParse = <T>(
  value?: string | null,
  fallback?: T,
  reviver?: (key: string, value: any, context?: { source: string }) => any,
): T | undefined => {
  try {
    if (value !== undefined && value !== null) {
      return JSON.parse(
        JSON.stringify(jsonBigIntToString.parse(value)),
        reviver,
      ) as T;
    }
  } catch (error) {
    return fallback;
  }
};

/**
 * 格式化 json string，为其添加换行和缩进
 */
export function formateJsonStr(jsonStr: string) {
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2);
  } catch (e) {
    return jsonStr;
  }
}

export function stringifyWithSortedKeys(
  obj: Record<string, any>,
  replacer?: (number | string)[] | null,
  space?: string | number,
) {
  if (!obj) {
    return undefined;
  }
  const sortedKeys = Object.keys(obj).sort();
  const orderedObj: Record<string, any> = {};
  sortedKeys.forEach(key => {
    orderedObj[key] = obj[key];
  });
  return JSON.stringify(orderedObj, replacer, space);
}

export function objSortedKeys(obj: Record<string, any>) {
  if (!obj) {
    return undefined;
  }
  const sortedKeys = Object.keys(obj).sort();
  const orderedObj: Record<string, any> = {};
  sortedKeys.forEach(key => {
    orderedObj[key] =
      typeof obj[key] === 'object' &&
      obj[key] !== null &&
      !Array.isArray(obj[key])
        ? objSortedKeys(obj[key])
        : obj[key];
  });
  return orderedObj;
}

/**
 * 仅校验字符串是否为合法 JSON 对象（{...} 格式）
 * 拒绝：单个值（111、"hello"、true）、数组（[1,2,3]）、非法 JSON
 * 允许：任意合法 JSON 对象（嵌套对象、对象内包含数组等）
 * @param {string} str - 要校验的字符串
 * @returns {boolean} 符合要求返回 true，否则 false
 */
export function isLegalJSONObject(str: string) {
  // 1. 先排除非字符串、空字符串、纯空格字符串
  if (typeof str !== 'string' || str.trim() === '') {
    return false;
  }

  try {
    const parsed = JSON.parse(str);
    // 2. 关键判断：解析结果必须是「对象」且「不是数组」（数组是 array 类型，不是纯对象）
    // 用 Object.prototype.toString 确保准确判断（避免 null、数组等误判）
    return Object.prototype.toString.call(parsed) === '[object Object]';
  } catch (error) {
    // 3. 解析报错（如 '000'、'{name:123}'）直接返回 false
    return false;
  }
}
