// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { ContentType, type Message } from '@cozeloop/api-schema/evaluation';

export function multiModelValidate(
  messages: Message[],
  model: Model | undefined,
): string | undefined {
  const hasMultiModelVar = messages?.some(
    message => message.content?.content_type === ContentType.MultiPart,
  );
  if (hasMultiModelVar && !model?.ability?.multi_modal) {
    return I18n.t('model_not_support_multimodal');
  }
  return;
}
