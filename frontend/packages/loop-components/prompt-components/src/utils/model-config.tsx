// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { Tag } from '@coze-arch/coze-design';

import { convertInt64ToNumber } from '@/model-config-editor/model-config-form';
import { DEFAULT_MAX_TOKENS } from '@/consts';

import { ModelStatus, type Model, type ModelConfig } from '../model-types';

export const convertModelToModelConfig = (model?: Model): ModelConfig => ({
  id: model?.id,
  name: model?.displayName,
  provider: model?.provider,
  provider_model_id: model?.identification,
  temperature: convertInt64ToNumber(model?.defaultRuntimeParam?.temperature),
  max_tokens: convertInt64ToNumber(
    model?.defaultRuntimeParam?.maxTokens || DEFAULT_MAX_TOKENS,
  ),
  top_p: convertInt64ToNumber(model?.defaultRuntimeParam?.topP),
  function_call_mode: model?.ability?.functionCallEnabled,
});

export const renderModelOfflineTag = (model?: Model) => {
  if (model?.modelStatus === ModelStatus.Offlining) {
    return (
      <Tag color="yellow" className="flex-shrink-0" size="mini">
        {I18n.t('prompt_being_deprecated')}
      </Tag>
    );
  } else if (model?.modelStatus === ModelStatus.Unavailable) {
    return (
      <Tag color="red" className="flex-shrink-0" size="mini">
        {I18n.t('deprecated')}
      </Tag>
    );
  }
  return null;
};
