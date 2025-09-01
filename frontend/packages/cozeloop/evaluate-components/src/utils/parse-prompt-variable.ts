import { common, type Message } from '@cozeloop/api-schema/evaluation';

import { extractDoubleBraceFields } from './double-brace';

/**
 * Prompt变量类型，字符串或多模态类型
 * TODO: 后续等PE支持多模态后替换为 prompt.VariableType
 */
export enum PromptVariableType {
  String = 'string',
  MultiPartVariable = 'multi_part_variable',
}

export interface MultiPartVariableContent {
  content_type?: common.ContentType.MultiPartVariable;
  multi_part?: common.Content[];
}

/**
 * Prompt变量，包含变量名和变量类型
 */
export interface PromptVariable {
  /** 变量名 */
  key: string;
  /** 变量类型，字符串或多模态类型 */
  type: PromptVariableType;
}

/**
 * 解析Prompt字符串中的变量
 * @param promptStr Prompt字符串
 * @returns Prompt变量列表
 */
export function parsePromptVariables(promptStr: string): PromptVariable[] {
  const vars = extractDoubleBraceFields(promptStr);
  const variables: PromptVariable[] = vars.map(variable => ({
    key: variable,
    type: PromptVariableType.String,
  }));
  return variables;
}

/**
 * 从消息列表中提取所有Prompt变量
 * @param messages 消息列表
 * @returns Prompt变量列表
 */
export function parseMessagesVariables(messages: Message[]) {
  const variables: PromptVariable[] = [];
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
              type: PromptVariableType.MultiPartVariable,
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
    if (nameMap.get(variable.key) === true) {
      return false;
    }
    nameMap.set(variable.key, true);
    return true;
  });
  return uniqueVariables;
}
