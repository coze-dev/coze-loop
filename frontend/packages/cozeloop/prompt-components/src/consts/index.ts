// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

import { I18n } from '@cozeloop/i18n-adapter';

export const VARIABLE_MAX_LEN = 50;

export const modelConfigLabelMap: Record<string, string> = {
  temperature: I18n.t('temperature'),
  max_tokens: I18n.t('max_tokens'),
  top_p: I18n.t('top_p'),
  top_k: I18n.t('top_k'),
  presence_penalty: I18n.t('presence_penalty'),
  frequency_penalty: I18n.t('frequency_penalty'),
};

export const DEFAULT_MAX_TOKENS = 4096;
