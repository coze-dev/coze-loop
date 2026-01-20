// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';

// 图片对象 schema
const imageSchema = z.object({
  url: z.string().optional(),
  name: z.string().optional(),
});

// 文件对象 schema
const fileSchema = z.object({
  url: z.string().optional(),
  name: z.string().optional(),
  suffix: z.string().optional(),
});

// 内容项 schema
const contentItemSchema = z.object({
  content_type: z.string(),
  text: z.string().optional(),
  image: z.union([imageSchema.optional(), z.null()]),
  file: z.union([fileSchema.optional(), z.null()]),
});

// 查询 span 输入/输出 schema
export const querySpanSchema = z.object({
  contents: z.array(contentItemSchema),
});

// 导出类型
export type Image = z.infer<typeof imageSchema>;
export type File = z.infer<typeof fileSchema>;
export type ContentItem = z.infer<typeof contentItemSchema>;
export type QuerySpanSchema = z.infer<typeof querySpanSchema>;
