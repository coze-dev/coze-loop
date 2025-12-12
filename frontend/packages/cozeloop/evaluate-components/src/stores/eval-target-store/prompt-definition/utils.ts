// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { ContentType, type FieldSchema } from '@cozeloop/api-schema/evaluation';

/** Prompt user_query 字段映射schema */
export const userQueryKeySchema: FieldSchema = {
  content_type: ContentType.Text,
  key: 'user_query',
  name: 'user_query',
};
