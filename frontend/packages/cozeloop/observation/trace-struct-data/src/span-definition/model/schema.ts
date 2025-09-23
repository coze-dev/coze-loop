// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';

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

// 工具函数参数 schema
const toolFunctionSchema = z.object({
  name: z.string(),
  arguments: z.string().optional(),
});

// 工具调用 schema
const toolCallSchema = z.object({
  type: z.string(),
  function: toolFunctionSchema,
});

// 工具定义参数属性 schema
const toolParameterPropertySchema = z.object({
  description: z.string().optional(),
  type: z.string().optional(),
});

// 工具定义参数 schema
const toolParametersSchema = z.object({
  required: z.array(z.string()).optional(),
  properties: z.record(toolParameterPropertySchema).optional(),
});

// 工具定义 schema
const toolSchema = z.object({
  type: z.string(),
  function: z.object({
    name: z.string(),
    description: z.string().optional(),
    parameters: z.union([z.null(), toolParametersSchema]),
  }),
});

// 消息部分内容 schema
const messagePartSchema = z.object({
  type: z.string(),
  text: z.string().optional(),
  image_url: imageUrlSchema.optional(),
  file_url: fileUrlSchema.optional(),
});

// 基础消息 schema
const messageSchema = z.object({
  role: z.string(),
  content: z.union([z.string(), z.null()]).optional(),
  reasoning_content: z.string().optional(),
  tool_calls: z.array(toolCallSchema).optional(),
  parts: z.array(messagePartSchema).optional(),
});

export const modelInputSchema = z.object({
  tools: z.array(toolSchema).optional(),
  messages: z.array(messageSchema),
});

// 模型输出的工具调用 schema（可能有 id 字段）
const outputToolCallSchema = z.object({
  type: z.string(),
  function: toolFunctionSchema,
});

// 模型输出的消息部分 schema
const outputMessagePartSchema = z.object({
  type: z.string(),
  text: z.string().optional(),
  file_url: fileUrlSchema.optional(),
  image_url: imageUrlSchema.optional(),
});

// 模型输出的消息 schema
const outputMessageSchema = z.object({
  role: z.string(),
  content: z.union([z.string(), z.null()]).optional(),
  reasoning_content: z.string().optional(),
  tool_calls: z.array(outputToolCallSchema).optional(),
  parts: z.array(outputMessagePartSchema).optional(),
});

// 选择项 schema
const choiceSchema = z.object({
  index: z.number().optional(),
  message: outputMessageSchema,
});

export const modelOutputSchema = z.object({
  choices: z.array(choiceSchema),
});

// 导出类型
export type FileUrl = z.infer<typeof fileUrlSchema>;
export type ImageUrl = z.infer<typeof imageUrlSchema>;
export type ToolFunction = z.infer<typeof toolFunctionSchema>;
export type ToolCall = z.infer<typeof toolCallSchema>;
export type ToolParameterProperty = z.infer<typeof toolParameterPropertySchema>;
export type ToolParameters = z.infer<typeof toolParametersSchema>;
export type Tool = z.infer<typeof toolSchema>;
export type MessagePart = z.infer<typeof messagePartSchema>;
export type Message = z.infer<typeof messageSchema>;
export type OutputMessage = z.infer<typeof outputMessageSchema>;
export type Choice = z.infer<typeof choiceSchema>;
export type ModelInputSchema = z.infer<typeof modelInputSchema>;
export type ModelOutputSchema = z.infer<typeof modelOutputSchema>;
