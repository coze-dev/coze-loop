// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any -- Normalization functions need to handle various input types */

import { type RawMessage } from '../../types';
import {
  type Choice,
  type ModelOutputSchema,
  type ResponseApiFunctionCall,
  type ModelInputSchema,
  type Tool as ModelTool,
  type OutputMessagePart,
} from './schema';

type ModelInputWithMessages = Extract<ModelInputSchema, { messages: unknown }>;
type ModelOutputWithChoices = Extract<ModelOutputSchema, { choices: unknown }>;
type ResponseApiModelInput = Extract<ModelInputSchema, { input?: unknown }>;
type ResponseApiModelOutput = Extract<ModelOutputSchema, { output?: unknown }>;

type ArrayElement<T> = T extends Array<infer U> ? U : never;

type ResponseApiOutputItem = ArrayElement<
  NonNullable<ResponseApiModelOutput['output']>
>;
type ResponseApiOutputMessageItem = Extract<
  ResponseApiOutputItem,
  { content?: unknown }
>;

export const normalizeTools = (
  tools: ModelInputSchema['tools'],
): ModelTool[] => {
  if (!tools || !Array.isArray(tools)) {
    return [];
  }

  return tools.map((tool: any) => {
    // 如果已经是符合toolSchema格式的工具，直接返回
    if ('function' in tool && typeof tool.function === 'object') {
      return tool as ModelTool;
    }

    // 格式化 Response API格式的工具
    if ('name' in tool) {
      return {
        type: tool.type || 'function',
        function: {
          name: tool.name,
          description: tool.description,
          parameters: tool.parameters,
        },
      } satisfies ModelTool;
    }

    return tool as ModelTool;
  });
};

const normalizeResponseApiInputParts = (
  content: OutputMessagePart[] | undefined | null | string,
) => {
  if (!Array.isArray(content)) {
    return content;
  }

  return content.map(part => ({
    type: part?.type ?? 'text',
    text: part?.text ?? part?.refusal ?? '',
    image_url: part?.image_url,
    file_url: part?.file_url,
  }));
};

const normalizeResponseApiOutputParts = (
  content: ResponseApiOutputMessageItem['content'] | undefined,
) => {
  if (!Array.isArray(content)) {
    return undefined;
  }
  return content.map(part => ({
    type: part?.type ?? 'output_text',
    text: part?.text ?? part?.refusal ?? '',
    image_url: part?.image_url,
    file_url: part?.file_url,
  }));
};

const isStandardModelInput = (
  data: ModelInputSchema,
): data is ModelInputWithMessages => 'messages' in data;

const isResponseApiModelInput = (
  data: ModelInputSchema,
): data is ResponseApiModelInput => 'input' in data;

const isStandardModelOutput = (
  data: ModelOutputSchema,
): data is ModelOutputWithChoices => 'choices' in data;

const isResponseApiModelOutput = (
  data: ModelOutputSchema,
): data is ResponseApiModelOutput => 'output' in data;

export const normalizeInputMessages = (
  data: ModelInputSchema,
  previousResponseId?: string | null,
): RawMessage[] => {
  if (isStandardModelInput(data)) {
    return (
      data.messages?.map(message => ({
        ...message,
        previous_response_id: previousResponseId ?? undefined,
      })) ?? []
    );
  }

  if (!isResponseApiModelInput(data)) {
    return [];
  }

  const responseInput = data.input;

  if (
    typeof responseInput === 'string' ||
    !Array.isArray(responseInput) ||
    !responseInput
  ) {
    return [{ role: '-', content: responseInput }];
  }

  return (
    responseInput?.map(message => {
      const normalizedParts = normalizeResponseApiInputParts(message?.content);

      return {
        role: message?.role ?? '',
        content: Array.isArray(message?.content)
          ? undefined
          : (normalizedParts ?? message?.content ?? ''),
        parts: Array.isArray(normalizedParts) ? normalizedParts : undefined,
      } satisfies RawMessage;
    }) ?? []
  );
};

const normalizeResponseApiOutputMessage = (
  item: ResponseApiOutputItem,
): Choice['message'] => {
  // 处理 reasoning 消息
  if ('summary' in item) {
    const normalizedParts = normalizeResponseApiOutputParts(item.content) ?? [];
    const summaryParts = (item.summary ?? []).map(summary => ({
      type: summary?.type ?? 'reasoning',
      text: summary?.text,
    }));

    return {
      role: item.type ?? 'reasoning',
      parts: [...normalizedParts, ...summaryParts],
    };
  }

  // 处理正常的消息
  if ('role' in item) {
    const normalizedParts = normalizeResponseApiOutputParts(item.content);
    return {
      role: item.role ?? '',
      content: typeof item.content === 'string' ? item.content : '',
      parts: normalizedParts,
    };
  }

  // 处理函数调用消息
  const functionCallItem = item as ResponseApiFunctionCall;
  const functionType = functionCallItem.type ?? 'function_call';

  return {
    role: functionType,
    tool_calls: [
      {
        type: functionType,
        function: {
          name: functionCallItem.name ?? '',
          arguments: functionCallItem.arguments,
        },
      },
    ],
  };
};

export const normalizeOutputChoices = (data: ModelOutputSchema): Choice[] => {
  if (isStandardModelOutput(data)) {
    return data.choices ?? [];
  }

  if (!isResponseApiModelOutput(data) || !data.output) {
    return [];
  }

  return data.output
    .filter(item => Object.keys(item).length > 0)
    .map((item, index) => ({
      index,
      message: normalizeResponseApiOutputMessage(item),
    }));
};
