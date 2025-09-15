import { useMemo } from 'react';

import { BasicModelConfigEditor } from '@cozeloop/prompt-components';
import { BasicCard } from '@cozeloop/components';
import { useModelList, useSpace } from '@cozeloop/biz-hooks-adapter';
import { type PromptRuntimeParam } from '@cozeloop/api-schema/evaluation';
import { type EvaluateTargetPromptDynamicParamsProps } from '@cozeloop/adapter-interfaces/evaluate';
import { Switch } from '@coze-arch/coze-design';

export function EvaluateTargetPromptDynamicParams(
  props: EvaluateTargetPromptDynamicParamsProps,
) {
  const { spaceIDWhenDemoSpaceItsPersonal } = useSpace();
  const model = useModelList(spaceIDWhenDemoSpaceItsPersonal);

  const parsedValue = useMemo(() => {
    if (!props.value?.json_value) {
      return {
        model_config: {},
      };
    }
    try {
      return JSON.parse(props.value.json_value) as PromptRuntimeParam;
    } catch (error) {
      return {
        model_config: {},
      };
    }
  }, [props.value]);

  if (props.disabled) {
    return (
      <BasicModelConfigEditor
        disabled={props.disabled}
        value={parsedValue.model_config}
        models={model.data?.models}
        modelSelectProps={{
          className: 'w-full',
          loading: model.loading,
          optionClassName: 'max-w-[750px]',
        }}
      />
    );
  }
  return (
    <BasicCard
      className="mt-4"
      title={
        <div>
          覆盖 Prompt 模型配置{' '}
          <Switch
            size="small"
            checked={!!props.value}
            onChange={v => {
              props.onChange?.(
                v
                  ? {
                      json_value: JSON.stringify({
                        model_config:
                          props.prompt?.prompt_commit?.detail?.model_config,
                      }),
                    }
                  : undefined,
              );
            }}
          />
        </div>
      }
    >
      {props.value ? (
        <BasicModelConfigEditor
          disabled={props.disabled}
          value={parsedValue.model_config}
          onChange={v => {
            props.onChange?.({
              ...props.value,
              json_value: JSON.stringify({ model_config: v }),
            });
          }}
          onModelChange={props.onModelChange}
          models={model.data?.models}
          modelSelectProps={{
            className: 'w-full',
            loading: model.loading,
            optionClassName: 'max-w-[750px]',
          }}
        />
      ) : null}
    </BasicCard>
  );
}
