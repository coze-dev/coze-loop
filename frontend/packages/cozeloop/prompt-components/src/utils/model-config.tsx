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
        下线中
      </Tag>
    );
  } else if (model?.modelStatus === ModelStatus.Unavailable) {
    return (
      <Tag color="red" className="flex-shrink-0" size="mini">
        已下线
      </Tag>
    );
  }
  return null;
};
