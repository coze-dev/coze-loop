// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { InputSlider } from '@cozeloop/components';
import { type Model } from '@cozeloop/api-schema/llm-manage';
import { Form, withField } from '@coze-arch/coze-design';

import { usePromptDevProviderContext } from '../prompt-develop/components/prompt-provider';
import { getDefaultModelConfig, getInputSliderConfig } from './utils';
import { FormModelThinkingControl } from './model-thinking-control';

const FormInputSlider = withField(InputSlider);

export function ModelConfigForm({ model }: { model: Model | undefined }) {
  const { modelInfo } = usePromptDevProviderContext();
  const paramSchemas = model?.param_config?.param_schemas ?? [];
  const defaultValues = useMemo(
    () =>
      model
        ? ((modelInfo?.getDefaultModelConfig ?? getDefaultModelConfig)?.(
            model,
          ) ?? {})
        : {},
    [model, modelInfo],
  );
  const paramsFields = paramSchemas?.map(item => item.name ?? '') ?? [];

  if (!model) {
    return null;
  }
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
      <FormModelThinkingControl
        paramSchemas={paramSchemas}
        noLabel
        field="param_config_values"
        fieldClassName="!py-0"
        labelPosition="left"
      />

      {modelInfo?.customModelFormItemsRender?.({ model, paramSchemas })}
    </>
  );
}
