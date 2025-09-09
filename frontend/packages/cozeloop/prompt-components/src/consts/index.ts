// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const VARIABLE_MAX_LEN = 50;

export const modelConfigLabelMap: Record<string, string> = {
  temperature: '生成随机性',
  max_tokens: '最大回复长度',
  top_p: 'Top P',
  top_k: 'Top K',
  presence_penalty: '存在惩罚',
  frequency_penalty: '频率惩罚',
};

export const DEFAULT_MAX_TOKENS = 4096;
