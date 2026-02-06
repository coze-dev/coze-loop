// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ExptTabDefinition } from './types';

// 根据 type 注册
const exptTabMap = new Map<string | number, ExptTabDefinition>();

/**
 * 注册评测对象选择器
 * @returns 注销评测对象选择器 方法
 */
const registerExptTab = (item: ExptTabDefinition) => {
  exptTabMap.set(item.type, item);
  return () => {
    exptTabMap.delete(item.type);
  };
};

/**
 * 获取 type 评测对象选择器
 * @param type 评测对象的value
 * @returns 评测对象选择器
 */
const getExptTab = (type: string | number) => exptTabMap.get(type);

const getExptTabList = () => Array.from(exptTabMap.values());

/**
 * 评测对象选择器
 */
export const useExptTab = () => ({
  getExptTab,
  registerExptTab,
  getExptTabList,
});
