// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';

import { PromptArgumentValueType } from './argument-value-render';

// 基础文件URL schema
const fileUrlSchema = z.object({
  name: z.string().optional(),
  url: z.string().optional(),
  detail: z.string().optional(),
  suffix: z.string().optional(),
});

// 基础图片URL schema
const imageUrlSchema = z.object({
  name: z.string().optional(),
  url: z.string().optional(),
  detail: z.string().optional(),
});

// 基础部分内容 schema
const partSchema = z.object({
  type: z.string(),
  text: z.string().optional(),
  image_url: imageUrlSchema.optional(),
  file_url: fileUrlSchema.optional(),
});

// 工具调用 schema
const toolCallSchema = z.object({
  id: z.string().optional(),
  type: z.string(),
  function: z.object({
    name: z.string(),
    arguments: z.string().optional(),
  }),
});

// 基础消息 schema
const baseMessageSchema = z.object({
  role: z.string(),
  content: z.union([z.string(), z.null()]).optional(),
  reasoning_content: z.string().optional(),
  parts: z.array(partSchema).optional(),
  name: z.string().optional(),
  tool_calls: z.array(toolCallSchema).optional(),
});

// 输入参数 schema
const inputArgumentSchema = z.array(
  z.object({
    key: z.string(),
    value: z
      .union([
        z.string(),
        z.object({ content: z.union([z.string(), z.null()]) }).optional(),
        z.array(partSchema).optional(),
      ])
      .optional(),
    source: z.string(),
    value_type: z
      .union([
        z.literal(PromptArgumentValueType.Text),
        z.literal(PromptArgumentValueType.Message),
        z.literal(PromptArgumentValueType.MessagePart),
      ])
      .optional(),
  }),
);

export const promptInputSchema = z.object({
  arguments: z.union([inputArgumentSchema, z.null()]).optional(),
  templates: z.array(baseMessageSchema),
});

// 输出消息 schema（与输入稍有不同）
const outputPartSchema = z.object({
  type: z.string().optional(),
  text: z.string().optional(),
  image_url: imageUrlSchema.optional(),
  file_url: fileUrlSchema.optional(),
});

const outputMessageSchema = z.object({
  role: z.string(),
  content: z.union([z.string(), z.null()]).optional(),
  reasoning_content: z.string().optional(),
  parts: z.array(outputPartSchema).optional(),
});

const messageArrayOrNullSchema = z.union([
  z.array(outputMessageSchema),
  z.null(),
]);

const userPromptOutputSchema = z.object({
  prompts: messageArrayOrNullSchema,
});

const servicePromptOutputSchema = messageArrayOrNullSchema;

export const promptOutputSchema = z.union([
  userPromptOutputSchema,
  servicePromptOutputSchema,
]);

// 导出类型
export type FileUrl = z.infer<typeof fileUrlSchema>;
export type ImageUrl = z.infer<typeof imageUrlSchema>;
export type Part = z.infer<typeof partSchema>;
export type ToolCall = z.infer<typeof toolCallSchema>;
export type BaseMessage = z.infer<typeof baseMessageSchema>;
export type OutputMessage = z.infer<typeof outputMessageSchema>;
export type PromptInputSchema = z.infer<typeof promptInputSchema>;
export type PromptOutputSchema = z.infer<typeof promptOutputSchema>;
export type MultiPartSchema = z.infer<typeof partSchema>;
