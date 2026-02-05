// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  prompt,
  VariableType,
  type VariableDef,
} from '@cozeloop/api-schema/prompt';
import {
  common,
  ContentType,
  type Content,
  type FieldSchema,
  type Message,
} from '@cozeloop/api-schema/evaluation';

import { extractDoubleBraceFields } from './double-brace';

export interface MultiPartVariableContent {
  content_type?: common.ContentType.MultiPartVariable;
  multi_part?: common.Content[];
}

/**
 * 解析Prompt字符串中的变量
 * @param promptStr Prompt字符串
 * @returns Prompt变量列表
 */
export function parsePromptVariables(promptStr: string): VariableDef[] {
  const vars = extractDoubleBraceFields(promptStr);
  const variables: VariableDef[] = vars.map(variable => ({
    key: variable,
    type: VariableType.String,
  }));
  return variables;
}

/**
 * 从消息列表中提取所有Prompt变量
 * @param messages 消息列表
 * @returns Prompt变量列表
 */
export function parseMessagesVariables(messages: Message[]) {
  const variables: VariableDef[] = [];
  messages?.forEach(message => {
    const contentType = message?.content?.content_type;
    if (contentType === common.ContentType.Text) {
      const str = message?.content?.text ?? '';
      const newVars = parsePromptVariables(str);
      variables.push(...newVars);
    } else if (contentType === common.ContentType.MultiPart) {
      const multiPart = message?.content?.multi_part;
      if (multiPart) {
        multiPart.forEach(item => {
          if (item?.content_type === common.ContentType.MultiPartVariable) {
            variables.push({
              type: VariableType.MultiPart,
              key: item?.text ?? '',
            });
          } else if (item?.content_type === common.ContentType.Text) {
            const newVars = parsePromptVariables(item?.text ?? '');
            variables.push(...newVars);
          }
        });
      }
    }
  });

  const nameMap = new Map<string, true>();
  const uniqueVariables = variables.filter(variable => {
    if (!variable.key) {
      return false;
    }
    if (nameMap.get(variable.key) === true) {
      return false;
    }
    nameMap.set(variable.key, true);
    return true;
  });
  return uniqueVariables;
}

/**
 * 从Prompt变量定义转换为评测集字段schema(包含type)
 * @param variableDef Prompt变量定义
 * @returns 字段schema
 */
export function promptVariableDefToFieldSchema(
  variableDef: prompt.VariableDef,
): FieldSchema & { type?: string } {
  const variableType = variableDef.type;
  const isMultiPart = variableType === prompt.VariableType.MultiPart;
  // TODO: @武文琦 这里不需要text_schema 使用type描述类型
  const fieldSchema: FieldSchema & { type?: string } = {
    name: variableDef.key,
    key: variableDef.key,
    description: variableDef.desc,
    type: isMultiPart ? ContentType.MultiPart : variableType,
    content_type: isMultiPart
      ? common.ContentType.MultiPart
      : common.ContentType.Text,
  };
  return fieldSchema;
}

export function promptPartsToMultiParts(
  parts: prompt.ContentPart[],
): Content[] {
  const multiParts = parts?.map(part => {
    const typeMap = {
      [prompt.ContentType.Text]: common.ContentType.Text,
      [prompt.ContentType.MultiPartVariable]:
        common.ContentType.MultiPartVariable,
    };
    const multiPart: Content = {
      content_type:
        typeMap[part.type as prompt.ContentType] ?? common.ContentType.Text,
      text: part.text,
    };
    return multiPart;
  });
  return multiParts;
}

export function promptMessageToEvalMessage(msg: prompt.Message) {
  const { role, content, parts } = msg ?? {};
  const isMultiPart = Array.isArray(parts) && parts.length > 0;
  const multiPart = isMultiPart ? promptPartsToMultiParts(parts) : undefined;
  const roleMap = {
    [prompt.Role.User]: common.Role.User,
    [prompt.Role.System]: common.Role.System,
    [prompt.Role.Assistant]: common.Role.Assistant,
    [prompt.Role.Tool]: common.Role.Tool,
  };
  const msgRole: Message['role'] = roleMap[role ?? ''] ?? role;
  const newM: Message = {
    role: msgRole,
    content: {
      content_type: isMultiPart
        ? common.ContentType.MultiPart
        : common.ContentType.Text,
    },
  };
  if (newM.content) {
    if (isMultiPart) {
      newM.content.multi_part = multiPart;
    } else {
      newM.content.text = content;
    }
  }
  return newM;
}
