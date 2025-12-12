/* eslint-disable complexity */
import { InfoTooltip } from '@cozeloop/components';
import { type ModelConfig } from '@cozeloop/api-schema/prompt';
import { type ParamSchema, type Model } from '@cozeloop/api-schema/llm-manage';
import { type LabelProps, Typography } from '@coze-arch/coze-design';
import { CalypsoLazy } from '@bytedance/calypso';

import { DEFAULT_MAX_TOKENS, modelConfigLabelMap } from '@/consts';

export interface ModelConfigWithName
  extends ModelConfig,
    Record<string, unknown | undefined> {
  model_name?: string;
}

const convertInt64ToNumber = (v?: Int64) => {
  if (v !== undefined) {
    const num = Number(v);
    return isNaN(num) ? v : num;
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
  > & {
    [key: string]: unknown | undefined;
  } = {};
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

export const getInputSliderConfig = (
  key: string,
  modelParams: ParamSchema[],
  customLabel?: React.ReactNode,
): {
  min?: number;
  max?: number;
  defaultValue?: number;
  label?: React.ReactNode | LabelProps;
  optionList?: Array<{ value?: string; label?: string }>;
} => {
  const param = modelParams.find(item => item.name === key);
  const max = key === 'max_tokens' ? DEFAULT_MAX_TOKENS : 0;
  if (!param) {
    return {};
  }

  return {
    min: Number(param?.min || 0),
    max: Math.max(Number(param?.max || 1), max),
    defaultValue: Number(param?.default_value ?? max),
    label: {
      text: (
        <Typography.Text
          ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
          className="!max-w-[100px]"
        >
          {customLabel ??
            (param?.label
              ? param?.label
              : modelConfigLabelMap[param?.name || ''] || '')}
        </Typography.Text>
      ),
      extra: param?.desc ? (
        <InfoTooltip
          content={
            <CalypsoLazy
              markDown={param?.desc || ''}
              style={{ color: '#fff', padding: 0 }}
            />
          }
          useQuestion
        />
      ) : null,
    },
    optionList: param.options,
  };
};
