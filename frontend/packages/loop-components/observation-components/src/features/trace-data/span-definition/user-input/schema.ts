// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';

// 图片URL schema
const imageUrlSchema = z.object({
  url: z.string().optional(),
  name: z.string().optional(),
  detail: z.string().optional(),
});

// 文件URL schema
const fileUrlSchema = z.object({
  url: z.string().optional(),
  suffix_type: z.string().optional(),
  file_name: z.string().optional(),
});

// 内容对象 schema
const contentSchema = z.object({
  text: z.string().optional(),
  image_url: z.union([imageUrlSchema.optional(), z.null()]),
  file_url: z.union([fileUrlSchema.optional(), z.null()]),
});

// 用户输入项 schema
const userInputItemSchema = z.object({
  content_type: z.string(),
  content: contentSchema,
});

// 用户输入 schema（数组形式）
export const userInputSchema = z.array(userInputItemSchema);

// 导出类型
export type ImageUrl = z.infer<typeof imageUrlSchema>;
export type FileUrl = z.infer<typeof fileUrlSchema>;
export type Content = z.infer<typeof contentSchema>;
export type UserInputItem = z.infer<typeof userInputItemSchema>;
export type UserInputSchema = z.infer<typeof userInputSchema>;
