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
  arguments: z.union([z.string(), z.record(z.unknown())]).optional(),
});

// 工具调用 schema
const toolCallSchema = z.object({
  type: z.string(),
  function: toolFunctionSchema,
});

// 嵌套的 tool 参数定义的 schema（可能有
const toolNestParameterPropertySchema = z.object({
  description: z.string().optional(),
  type: z.string().optional(),
  properties: z.any().optional(),
});

// 工具定义参数 schema
export const toolParametersSchema = z.object({
  required: z.array(z.string()).optional(),
  properties: z.record(toolNestParameterPropertySchema).optional(),
});

// 工具定义 schema
const toolSchema = z.object({
  type: z.string(),
  function: z.object({
    name: z.string().optional(),
    description: z.string().optional(),
    parameters: toolParametersSchema.optional().nullable(),
  }),
});

// 消息内容的基础字段
const baseMessageContentSchema = z.object({
  text: z.string().optional(),
  image_url: imageUrlSchema.optional(),
  file_url: fileUrlSchema.optional(),
});

// 消息部分内容 schema
const messagePartSchema = baseMessageContentSchema.extend({
  type: z.string(),
});

// 基础消息 schema
const messageSchema = z.object({
  role: z.string(),
  content: z
    .union([z.string(), z.array(messagePartSchema)])
    .optional()
    .nullable(),
  reasoning_content: z.string().optional(),
  tool_calls: z.array(toolCallSchema).optional(),
  parts: z.array(messagePartSchema).optional(),
});

// 模型输出的工具调用 schema（可能有 id 字段）
const outputToolCallSchema = z.object({
  type: z.string(),
  function: toolFunctionSchema,
  id: z.string().optional(),
});

// 含 refusal 的扩展消息内容 schema
const extendedMessagePartSchema = baseMessageContentSchema.extend({
  type: z.string().optional(),
  refusal: z.string().optional(),
});

// 模型输出的消息部分 schema
const outputMessagePartSchema = extendedMessagePartSchema;

// 模型输出的消息 schema
const outputMessageSchema = z.object({
  role: z.string().optional(),
  content: z
    .union([z.string(), z.array(outputMessagePartSchema)])
    .optional()
    .nullable(),
  reasoning_content: z.string().optional(),
  tool_calls: z.array(outputToolCallSchema).optional().nullable(),
  parts: z.array(outputMessagePartSchema).optional().nullable(),
});

// 选择项 schema
const choiceSchema = z.object({
  index: z.number().optional(),
  message: outputMessageSchema,
});

// Response API 的 function call schema
export const responseApiFunctionCallSchema = z.object({
  type: z.string().optional(),
  id: z.string().optional(),
  call_id: z.string().optional(),
  name: z.string().optional(),
  arguments: z.string().optional(),
});

export const responseApiReasoningSchema = z.object({
  id: z.string().optional(),
  type: z.string().optional(),
  role: z.string().optional(),
  content: z.array(outputMessagePartSchema).nullable().optional(),
  summary: z
    .array(
      z.object({
        type: z.string().optional(),
        text: z.string().optional(),
      }),
    )
    .optional(),
});

// Response API 的 output 项 schema
const responseApiOutputItemSchema = z.union([
  responseApiReasoningSchema,
  responseApiFunctionCallSchema,
]);

// Response API 主 schema
export const modelOutputResponseApiSchema = z.object({
  id: z.string(),
  output: z.array(responseApiOutputItemSchema).nullable().optional(),
});

export const modelOutputSchema = z.union([
  z.object({
    choices: z.array(choiceSchema),
  }),
  modelOutputResponseApiSchema,
]);

// Response API input content 项 schema
const responseApiInputContentItemSchema = extendedMessagePartSchema;

// Response API input message schema
const responseApiInputMessageSchema = z.object({
  role: z.string().optional(),
  content: z
    .union([z.string(), z.array(responseApiInputContentItemSchema)])
    .nullable()
    .optional(),
});

// Response API input tool schema
const responseApiInputToolSchema = z.object({
  type: z.string().optional(),
  name: z.string().optional(),
  description: z.string().optional(),
  parameters: z.union([z.record(z.unknown()), z.string()]).optional(),
});

// Response API input 主 schema
export const modelInputResponseApiSchema = z.object({
  previous_response_id: z.string().optional().nullable(),
  input: z.union([z.array(responseApiInputMessageSchema), z.string()]),
  tools: z.array(responseApiInputToolSchema).nullable().optional(),
});

export const modelInputSchema = z.union([
  z.object({
    tools: z.array(toolSchema).optional(),
    messages: z.array(messageSchema),
    previous_response_id: z.string().optional().nullable(),
  }),
  modelInputResponseApiSchema,
]);

// 导出类型
export type FileUrl = z.infer<typeof fileUrlSchema>;
export type ImageUrl = z.infer<typeof imageUrlSchema>;
export type ToolFunction = z.infer<typeof toolFunctionSchema>;
export type ToolCall = z.infer<typeof toolCallSchema>;
// export type ToolParameterProperty = z.infer<typeof toolParameterPropertySchema>;
export type ToolParameters = z.infer<typeof toolParametersSchema>;
export type Tool = z.infer<typeof toolSchema>;
export type MessagePart = z.infer<typeof messagePartSchema>;
export type Message = z.infer<typeof messageSchema>;
export type OutputMessage = z.infer<typeof outputMessageSchema>;
export type Choice = z.infer<typeof choiceSchema>;
export type ModelInputSchema = z.infer<typeof modelInputSchema>;
export type ModelOutputSchema = z.infer<typeof modelOutputSchema>;
export type ResponseApiFunctionCall = z.infer<
  typeof responseApiFunctionCallSchema
>;
export type OutputMessagePart = z.infer<typeof outputMessagePartSchema>;
export type ModelNestParameterProperty = z.infer<
  typeof toolNestParameterPropertySchema
>;
