// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
const VARIABLE_MAX_LEN = 50;
const regex = new RegExp(`{{([a-zA-Z]\\w{0,${VARIABLE_MAX_LEN - 1}})}}`, 'gm');

/**
 * Extract all fields enclosed by double curly braces from a string.
 * @param str - The input string.
 * @returns An array of strings representing the extracted fields.
 */
export function extractDoubleBraceFields(str: string): string[] {
  const matches: string[] = [];
  let match = regex.exec(str);
  while (match !== null) {
    matches.push(match[1]);
    match = regex.exec(str);
  }
  return matches;
}

export interface SplitItem {
  text: string;
  isDoubleBrace?: boolean;
}
/**
 * 根据双花括号 {{}} 对输入的字符串进行分割，将分割后的部分存储在对象数组中返回。
 * 每个对象包含一个 text 属性，表示分割后的文本，以及一个可选的 isDoubleBrace 属性，
 * 用于标记该部分是否为双花括号包裹的内容。
 * @param str - 输入的需要进行分割的字符串。
 * @returns 一个对象数组，每个对象包含 text 属性和可选的 isDoubleBrace 属性。
 */
export function splitStringByDoubleBrace(str: string): SplitItem[] {
  const result: SplitItem[] = [];
  let lastIndex = 0;
  let match = regex.exec(str);
  // 循环执行正则匹配
  while (match !== null) {
    // 如果匹配结果之前有非匹配的字符，将其添加到结果数组中
    if (match.index > lastIndex) {
      result.push({ text: str.slice(lastIndex, match.index) });
    }
    // 将匹配到的数字添加到结果数组中
    result.push({ text: match[0], isDoubleBrace: true });
    // 更新 lastIndex 为当前匹配结果的末尾索引
    lastIndex = regex.lastIndex;

    match = regex.exec(str);
  }
  // 如果字符串末尾还有非匹配的字符，将其添加到结果数组中
  if (lastIndex < str.length) {
    result.push({ text: str.slice(lastIndex) });
  }
  return result;
}
