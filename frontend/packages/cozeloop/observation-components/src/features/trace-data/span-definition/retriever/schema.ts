// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';

// 文档对象 schema
const documentSchema = z.object({
  id: z.string().optional(),
  index: z.string().optional(),
  content: z.string(),
  vector: z.union([z.array(z.number()).optional(), z.null()]),
  score: z.number().optional(),
});

// Retriever 输入 schema
export const retrieverInputSchema = z.object({
  query: z.string(),
});

// Retriever 输出 schema
export const retrieverOutputSchema = z.object({
  documents: z.array(documentSchema),
});

// 导出类型
export type Document = z.infer<typeof documentSchema>;
export type RetrieverInputSchema = z.infer<typeof retrieverInputSchema>;
export type RetrieverOutputSchema = z.infer<typeof retrieverOutputSchema>;
