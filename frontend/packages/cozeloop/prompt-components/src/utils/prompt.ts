// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ContentPart,
  ContentType,
  type Message,
  Role,
  type VariableDef,
  VariableType,
} from '@cozeloop/api-schema/prompt';

export const getPlaceholderErrorContent = (
  message?: Message,
  variables?: VariableDef[],
) => {
  if (message?.role === Role.Placeholder) {
    if (!message?.content) {
      return 'Placeholder 变量名不能为空';
    }
    if (!/^[A-Za-z][A-Za-z0-9_]*$/.test(message?.content)) {
      return '只允许输入英文、数字及下划线且首字母需为英文';
    }
    const normalVariables = variables?.filter(
      it => it.type !== VariableType.Placeholder,
    );
    const hasSameKey = normalVariables?.find(it => it.key === message?.content);
    if (hasSameKey) {
      return 'Placeholder 变量名不能与其他类型变量名重复，请修改 Placeholder 变量名';
    }
  }
  return '';
};

/**
 * 拆分多模态变量内容
 * @param content 包含多模态变量标签的内容
 * @returns 拆分后的数组，每个元素包含 type 和 text 属性
 */
export const splitMultimodalContent = (content: string) => {
  const result: Array<{ type: ContentType; text: string }> = [];

  // 使用正则表达式匹配 <multimodal-variable>xxx</multimodal-variable> 标签
  const regex = /<multimodal-variable>(.*?)<\/multimodal-variable>/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while (true) {
    match = regex.exec(content);
    if (!match) {
      break;
    }
    // 添加标签前的文本（如果有的话）
    if (match.index > lastIndex) {
      const textBefore = content.slice(lastIndex, match.index);
      if (textBefore) {
        result.push({ type: ContentType.Text, text: textBefore });
      }
    }

    // 添加多模态变量内容
    result.push({ type: ContentType.MultiPartVariable, text: match[1] });

    // 更新索引位置
    lastIndex = match.index + match[0].length;
  }

  // 添加标签后的剩余文本（如果有的话）
  if (lastIndex < content.length && result.length) {
    const textAfter = content.slice(lastIndex);
    if (textAfter) {
      result.push({ type: ContentType.Text, text: textAfter });
    }
  }

  // 如果没有匹配到任何标签，返回原始文本
  if (result.length === 0) {
    return [];
  }

  return result;
};

export const multimodalPartsToContent = (parts: ContentPart[]) => {
  const newPartsText = parts.map(part => {
    if (part.type === ContentType.MultiPartVariable) {
      return `<multimodal-variable>${part.text}</multimodal-variable>`;
    }
    return part.text;
  });
  return newPartsText.join('');
};

export function getMultimodalVariableText(variableName: string) {
  return `<multimodal-variable>${variableName}</multimodal-variable>`;
}
