// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-nocheck
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/consistent-type-assertions */
import { useEffect, useMemo, useRef } from 'react';

import { isEqual } from 'lodash-es';
import classNames from 'classnames';
import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { InfoTooltip, InputSlider } from '@cozeloop/components';
import { type ParamConfigValue } from '@cozeloop/api-schema/prompt';
import { type ParamSchema } from '@cozeloop/api-schema/llm-manage';
import {
  Form,
  type FormApi,
  FormSelect,
  Typography,
  withField,
} from '@coze-arch/coze-design';

import { getInputSliderConfig } from './utils';

const FormInputSlider = withField(InputSlider);

export type ParamConfigValuesType = Array<ParamConfigValue>;

export function ModelThinkingControl({
  value,
  onChange = () => console.log,
  disabled = false,
  paramSchemas = [],
}: {
  value?: ParamConfigValuesType;
  onChange?: (values: ParamConfigValuesType) => void;
  disabled?: boolean;
  paramSchemas?: ParamSchema[];
}) {
  const debounceChange = useDebounceFn(onChange, { wait: 200 });
  const formApiRef = useRef<FormApi<Record<string, unknown>>>();

  const initialValues = useMemo(
    () =>
      value?.reduce(
        (prev, cur) => {
          const number = Number(cur.value?.value);
          if (cur.name) {
            prev[cur.name] = isNaN(number) ? cur.value?.value : number;
          }

          return prev;
        },
        {} as Record<string, unknown>,
      ),
    [value],
  );

  const paramsFields = paramSchemas?.map(item => item.name ?? '') ?? [];
  const handleValueChange = allValues => {
    if (isEqual(allValues, initialValues)) {
      return;
    }

    const array = Object.keys(allValues).map(key => {
      const param = paramSchemas.find(item => item.name === key);
      return {
        name: key,
        label: param?.label,
        value: {
          value: `${allValues[key]}`,
          label: param?.options?.find(
            item => item.value === `${allValues[key]}`,
          )?.label,
        },
      };
    });
    debounceChange.run?.(array);
  };

  useEffect(() => {
    formApiRef.current?.setValues(initialValues || {});
  }, [initialValues]);

  const hasReasoningEffort = paramsFields.includes('reasoning_effort');

  return paramsFields.includes('thinking_type') ? (
    <div>
      <div className="flex items-center gap-1">
        <Typography.Text
          style={{
            marginTop: 8,
            marginBottom: 8,
            fontWeight: 500,
            display: 'block',
          }}
        >
          {I18n.t('prompt_deep_thinking')}
        </Typography.Text>
        <InfoTooltip
          content={I18n.t('prompt_deep_thinking_description')}
          useQuestion
        />
      </div>
      <Form
        labelWidth={120}
        getFormApi={formApi => (formApiRef.current = formApi)}
        onValueChange={handleValueChange}
        labelPosition="left"
      >
        {hasReasoningEffort ? (
          <div className="w-full text-right">
            <FormSelect
              {...(getInputSliderConfig(
                'reasoning_effort',
                paramSchemas,
              ) as any)}
              field="reasoning_effort"
              labelPosition="left"
              fieldClassName="!py-[4px]"
              disabled={disabled}
              className="w-[90px]"
            />
          </div>
        ) : (
          <>
            <div className="w-full text-right">
              <FormSelect
                {...(getInputSliderConfig(
                  'thinking_type',
                  paramSchemas,
                ) as any)}
                labelPosition="left"
                field="thinking_type"
                fieldClassName="!py-[4px]"
                disabled={disabled}
                className="w-[90px]"
              />
            </div>
            {paramsFields.includes('max_completion_tokens') ? (
              <div
                className={classNames({
                  hidden: initialValues?.thinking_type === 'disabled',
                })}
              >
                <FormInputSlider
                  {...(getInputSliderConfig(
                    'max_completion_tokens',
                    paramSchemas,
                    I18n.t('prompt_deep_thinking_length'),
                  ) as any)}
                  field="max_completion_tokens"
                  labelPosition="left"
                  fieldClassName="!py-[4px]"
                  disabled={disabled}
                />
              </div>
            ) : null}
          </>
        )}
      </Form>
    </div>
  ) : null;
}

export const FormModelThinkingControl = withField(
  ModelThinkingControl,
) as ReturnType<typeof withField>;
