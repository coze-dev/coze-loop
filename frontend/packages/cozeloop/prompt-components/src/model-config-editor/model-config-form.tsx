import { isUndefined } from 'lodash-es';
import { InputSlider } from '@cozeloop/components';
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

import { type Model, type ModelParam } from '../model-types';

export const convertInt64ToNumber = (v?: Int64) => {
  if (v !== undefined) {
    return Number(v);
  } else {
    return undefined;
  }
};

export const getInputSliderConfig = (
  key: string,
  modelParams: ModelParam[],
): {
  min?: number;
  max?: number;
  defaultValue?: number;
  label?: React.ReactNode | LabelProps;
} => {
  const param = modelParams.find(item => item.name === key);
  const max = key === 'max_tokens' ? DEFAULT_MAX_TOKENS : 0;

  return {
    min: Number(param?.min || 0),
    max: Math.max(Number(param?.max || 1), max),
    defaultValue: Number(param?.defaultVal || max),
    label: {
      text: (
        <Typography.Text>
          {param?.name ? modelConfigLabelMap[param.name] || '' : ''}
        </Typography.Text>
      ),
      extra: (
        <Tooltip
          content={
            <MdBoxLazy className="!text-white" markDown={param?.desc || ''} />
          }
          theme="dark"
        >
          <IconCozQuestionMarkCircle />
        </Tooltip>
      ),
    },
  };
};

const FormInputSlider = withField(InputSlider);
export function ModelConfigForm({ model }: { model?: Model }) {
  if (!model) {
    return null;
  }
  const modelAbility = model?.ability;
  const modelParams = modelAbility?.modelParams || [];
  const defaultRuntimeParam = model?.defaultRuntimeParam;
  return (
    <>
      <FormInputSlider
        field="max_tokens"
        labelPosition="left"
        {...getInputSliderConfig('max_tokens', modelParams)}
        label={{
          text: <Typography.Text>最大回复长度</Typography.Text>,
          extra: (
            <Tooltip
              content={
                <MdBoxLazy
                  className="!text-white"
                  markDown={
                    '- **max_tokens**: 控制模型输出的 Tokens 长度上限。通常 100 Tokens 约等于 150 个中文汉字。'
                  }
                />
              }
              theme="dark"
            >
              <IconCozQuestionMarkCircle />
            </Tooltip>
          ),
        }}
      />

      {isUndefined(defaultRuntimeParam?.temperature) ? null : (
        <FormInputSlider
          field="temperature"
          labelPosition="left"
          {...getInputSliderConfig('temperature', modelParams)}
          step={0.01}
        />
      )}

      {isUndefined(defaultRuntimeParam?.topP) ? null : (
        <FormInputSlider
          field="top_p"
          labelPosition="left"
          label={{
            text: <Typography.Text>Top P</Typography.Text>,
          }}
          {...getInputSliderConfig('top_p', modelParams)}
          step={0.01}
        />
      )}
      {modelAbility?.jsonModeEnabled ? (
        <Form.Switch
          labelPosition="left"
          label={{
            text: <Typography.Text>JSON Mode</Typography.Text>,
          }}
          field="json_mode"
        />
      ) : null}
    </>
  );
}
