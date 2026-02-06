// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { InputSlider } from '@cozeloop/components';
import { type ParamSchema, type Model } from '@cozeloop/api-schema/llm-manage';
import { IconCozQuestionMarkCircle } from '@coze-arch/coze-design/icons';
import {
  Form,
  type LabelProps,
  Tooltip,
  Typography,
  withField,
} from '@coze-arch/coze-design';
import { MdBoxLazy } from '@coze-arch/bot-md-box-adapter/lazy';

import { DEFAULT_MAX_TOKENS, modelConfigLabelMap } from '@/consts';

import { getDefaultModelConfig } from './utils';

const FormInputSlider = withField(InputSlider);

export const getInputSliderConfig = (
  key: string,
  modelParams: ParamSchema[],
): {
  min?: number;
  max?: number;
  defaultValue?: number;
  label?: React.ReactNode | LabelProps;
} => {
  const param = modelParams.find(item => item.name === key);
  const max = key === 'max_tokens' ? DEFAULT_MAX_TOKENS : 0;
  if (!param) {
    return {};
  }
  return {
    min: Number(param?.min || 0),
    max: Math.max(Number(param?.max || 1), max),
    defaultValue: Number(param?.default_value || max),
    label: {
      text: (
        <Typography.Text>
          {param?.label
            ? param?.label
            : modelConfigLabelMap[param?.name || ''] || ''}
        </Typography.Text>
      ),

      extra: param?.desc ? (
        <Tooltip
          content={
            <MdBoxLazy markDown={param?.desc || ''} style={{ color: '#fff' }} />
          }
          theme="dark"
        >
          <IconCozQuestionMarkCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] cursor-pointer" />
        </Tooltip>
      ) : null,
    },
  };
};

export function ModelConfigFormCommunity({
  model,
}: {
  model: Model | undefined;
}) {
  if (!model) {
    return null;
  }
  const paramSchemas = model.param_config?.param_schemas ?? [];
  const defaultValues = useMemo(
    () => (model ? getDefaultModelConfig(model) : {}),
    [model],
  );

  const paramsFields =
    model.param_config?.param_schemas?.map(item => item.name ?? '') ?? [];
  return (
    <>
      {paramsFields.includes('max_tokens') ? (
        <FormInputSlider
          label={I18n.t('max_tokens')}
          {...getInputSliderConfig('max_tokens', paramSchemas)}
          field="max_tokens"
          labelPosition="left"
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('temperature') ? (
        <FormInputSlider
          label={I18n.t('temperature')}
          {...getInputSliderConfig('temperature', paramSchemas)}
          field="temperature"
          labelPosition="left"
          step={0.01}
          defaultValue={defaultValues.temperature}
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('top_p') ? (
        <FormInputSlider
          label="Top P"
          {...getInputSliderConfig('top_p', paramSchemas)}
          field="top_p"
          labelPosition="left"
          step={0.01}
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('top_k') ? (
        <FormInputSlider
          label="Top K"
          {...getInputSliderConfig('top_k', paramSchemas)}
          field="top_k"
          labelPosition="left"
          step={1}
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('frequency_penalty') ? (
        <FormInputSlider
          label="Frequency Penalty"
          {...getInputSliderConfig('frequency_penalty', paramSchemas)}
          field="frequency_penalty"
          labelPosition="left"
          step={0.01}
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('presence_penalty') ? (
        <FormInputSlider
          label="Presence Penalty"
          {...getInputSliderConfig('presence_penalty', paramSchemas)}
          field="presence_penalty"
          labelPosition="left"
          step={0.01}
          fieldClassName="!py-[4px]"
        />
      ) : null}
      {paramsFields.includes('json_mode') ? (
        <Form.Switch
          label="JSON Mode"
          {...getInputSliderConfig('json_mode', paramSchemas)}
          labelPosition="left"
          field="json_mode"
          fieldClassName="!py-[4px]"
        />
      ) : null}
    </>
  );
}
