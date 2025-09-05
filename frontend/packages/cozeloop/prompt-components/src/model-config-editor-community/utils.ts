import { type ModelConfig } from '@cozeloop/api-schema/prompt';
import { type Model } from '@cozeloop/api-schema/llm-manage';

import { DEFAULT_MAX_TOKENS } from '@/consts';

export interface ModelConfigWithName extends ModelConfig {
  model_name?: string;
}

const convertInt64ToNumber = (v?: Int64) => {
  if (v !== undefined) {
    return Number(v);
  } else {
    return undefined;
  }
};

/** 从模型中读取默认配置 */
export function getDefaultModelConfig(model: Model) {
  const paramDefaultValue: Pick<
    ModelConfig,
    | 'temperature'
    | 'max_tokens'
    | 'top_p'
    | 'top_k'
    | 'frequency_penalty'
    | 'presence_penalty'
  > = {};
  if (model?.param_config?.param_schemas) {
    model?.param_config?.param_schemas?.forEach(item => {
      if (item.name && item.default_value !== undefined) {
        paramDefaultValue[item.name] = convertInt64ToNumber(item.default_value);
      }
      if (
        item.name === 'max_tokens' &&
        (item.default_value === undefined || item.default_value === '')
      ) {
        paramDefaultValue.max_tokens = DEFAULT_MAX_TOKENS;
      }
    });
  }
  return paramDefaultValue;
}

export const convertModelToModelConfig = (
  model: Model,
): ModelConfigWithName => {
  const paramDefaultValue = getDefaultModelConfig(model);
  const modelConfig: ModelConfigWithName = {
    model_name: model?.name,
    model_id: model?.model_id?.toString(),
    temperature: paramDefaultValue.temperature,
    max_tokens: paramDefaultValue.max_tokens,
    top_p: paramDefaultValue.top_p,
    top_k: paramDefaultValue.top_k,
    frequency_penalty: paramDefaultValue.frequency_penalty,
    presence_penalty: paramDefaultValue.presence_penalty,
  };
  return modelConfig;
};
