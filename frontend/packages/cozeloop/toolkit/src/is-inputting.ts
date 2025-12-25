// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/**
 * 是否正在输入，根据是否聚焦在可输入元素判断
 * @returns boolean
 */
export const isInputting = () => {
  const ele = document.activeElement;
  const inputTags = ['input', 'textarea'];
  if (ele) {
    const contentEditable = ele.getAttribute('contenteditable') === 'true';
    const tagName = ele.tagName.toLocaleLowerCase() || '';
    if (inputTags.includes(tagName) || contentEditable) {
      return true;
    }
  }
  return false;
};
