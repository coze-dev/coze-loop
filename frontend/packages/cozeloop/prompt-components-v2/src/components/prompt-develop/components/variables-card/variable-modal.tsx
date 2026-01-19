// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable security/detect-non-literal-regexp */

import { useEffect, useRef } from 'react';

import classNames from 'classnames';
import Ajv from 'ajv';
import { safeJsonParse } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  CodeMirrorJsonEditor,
  formateDecimalPlacesString,
} from '@cozeloop/components';
import {
  VariableType,
  type VariableDef,
  type VariableVal,
} from '@cozeloop/api-schema/prompt';
import {
  Form,
  Modal,
  Radio,
  RadioGroup,
  TextArea,
  useFormApi,
  withField,
  Toast,
  FormInput,
  FormSelect,
  type FormApi,
  CozInputNumber,
  Typography,
} from '@coze-arch/coze-design';

import { VARIABLE_MAX_LEN, VARIABLE_TYPE_ARRAY_MAP } from '@/consts';

import styles from './index.module.less';

interface AddVariablesProps {
  visible: boolean;
  data?: VariableVal;
  variableList?: VariableDef[];
  typeDisabled?: boolean;
  onCancel: () => void;
  onOk: (def: VariableDef, value: VariableVal, isEdit?: boolean) => void;
}

export const getSchemaErrorInfo = (errors: Object | null | undefined) => {
  if (!errors) {
    return I18n.t(
      'the_input_does_not_match_the_field_definition_of_the_column',
    );
  }
  const errorInfo = errors?.[0];
  const type = errorInfo?.keyword;
  const instancePath = errorInfo?.instancePath;
  switch (type) {
    case 'type': {
      return `${I18n.t('cozeloop_open_evaluate_data_type_mismatch_instancepath', { instancePath })}`;
    }
    case 'required': {
      return `${I18n.t('cozeloop_open_evaluate_missing_required_field', { placeholder1: instancePath ? `${instancePath}/` : '', placeholder2: errorInfo?.params?.missingProperty })}`;
    }
    case 'additionalProperties': {
      return `${I18n.t('cozeloop_open_evaluate_redundant_field_exists', { placeholder1: errorInfo?.params?.additionalProperty })}`;
    }
    default: {
      return I18n.t(
        'the_input_does_not_match_the_field_definition_of_the_column',
      );
    }
  }
};

const ajv = new Ajv();

const validateJson = (value: string, typeValue: VariableType) => {
  const data = safeJsonParse(value);
  if (typeof data !== 'object') {
    return I18n.t('the_input_content_is_not_in_legal_json_format');
  }

  const schema =
    typeValue === VariableType.Object
      ? {
          type: 'object',
          properties: {},
          additionalProperties: true,
        }
      : {
          type: 'array',
          items: {
            type: 'object',
          },
        };
  switch (typeValue) {
    case VariableType.Array_Boolean:
      schema.items = { type: 'boolean' };
      break;
    case VariableType.Array_Integer:
      schema.items = { type: 'integer' };
      break;
    case VariableType.Array_Float:
      schema.items = { type: 'number' };
      break;
    case VariableType.Array_String:
      schema.items = { type: 'string' };
      break;
    default:
      break;
  }

  const validate = ajv.compile(schema);
  const valid = validate(data);

  if (!valid) {
    return getSchemaErrorInfo(validate.errors);
  }
  return '';
};

export function VariableValueInput({
  value,
  disabled,
  editerHeight,
  typeValue,
  inputConfig,
  onChange,
  minHeight,
  maxHeight,
}: {
  value?: string;
  typeValue?: VariableType;
  disabled?: boolean;
  editerHeight?: number;
  minHeight?: number;
  maxHeight?: number;
  inputConfig?: {
    borderless?: boolean;
    inputClassName?: string;
    size?: 'small' | 'default';
    onFocus?: () => void;
    onBlur?: () => void;
  };
  onChange?: (v: string) => void;
}) {
  const formApi = useFormApi();

  const handleObjectEditorChange = changeValue => {
    onChange?.(changeValue);
    if (Object.keys(formApi).length) {
      if (!changeValue) {
        formApi?.setError('value', '');
      } else {
        const error = validateJson(
          changeValue,
          typeValue || VariableType.Object,
        );
        formApi?.setError('value', error);
      }
    }
  };

  if (
    typeValue === VariableType.Placeholder ||
    typeValue === VariableType.MultiPart
  ) {
    return null;
  }

  if (typeValue === VariableType.Boolean) {
    return (
      <RadioGroup
        onChange={e => onChange?.(e.target.value)}
        value={value}
        disabled={disabled}
      >
        <Radio value="true">True</Radio>
        <Radio value="false">False</Radio>
      </RadioGroup>
    );
  }

  if (typeValue === VariableType.Integer || typeValue === VariableType.Float) {
    return (
      <CozInputNumber
        key={typeValue}
        placeholder={
          typeValue === VariableType.Integer
            ? I18n.t('enter_integer')
            : I18n.t('prompt_please_input_float_max_4_decimal')
        }
        style={{ width: '100%' }}
        value={value}
        onChange={v => {
          // 使用正则表达式检查是否为有效数字（不包括科学记数法）
          const isValidNumber = /^-?(?:\d+\.?\d*|\.\d+)$/.test(`${v}`);
          if (!isValidNumber) {
            formApi?.setError?.(
              'value',
              I18n.t(
                'the_input_does_not_match_the_field_definition_of_the_column',
              ),
            ); // 设置错误信息
          } else {
            formApi?.setError?.('value', ''); // 清除错误信息
          }
          onChange?.(`${v}`);
        }}
        disabled={disabled}
        formatter={inputValue =>
          formateDecimalPlacesString(
            inputValue,
            Number(value),
            typeValue === VariableType.Integer ? 0 : 4,
          )
        }
        precision={typeValue === VariableType.Integer ? 0 : undefined}
        borderless={inputConfig?.borderless}
        className={inputConfig?.inputClassName}
        onFocus={inputConfig?.onFocus}
        onBlur={inputConfig?.onBlur}
        hideButtons
        size={inputConfig?.size}
      />
    );
  }

  if (typeValue === VariableType.Object || typeValue?.includes('array')) {
    return (
      <div
        className={classNames('rounded-[6px]', {
          'border border-solid border-[rgba(68,83,130,0.25)]':
            !inputConfig?.borderless,
        })}
        key={typeValue}
      >
        <CodeMirrorJsonEditor
          value={value || ''}
          onChange={handleObjectEditorChange}
          borderRadius={6}
          editerHeight={editerHeight}
          minHeight={minHeight}
          maxHeight={maxHeight}
          readonly={disabled}
          onFocus={inputConfig?.onFocus}
          onBlur={inputConfig?.onBlur}
        />
      </div>
    );
  }

  return (
    <TextArea
      key={typeValue}
      value={value}
      onChange={e => onChange?.(e)}
      placeholder={I18n.t('prompt_please_input_variable_value')}
      autosize={{
        minRows: 1,
        maxRows: 3,
      }}
      disabled={!typeValue || disabled}
      borderless={inputConfig?.borderless}
      className={inputConfig?.inputClassName}
      onFocus={inputConfig?.onFocus}
      onBlur={inputConfig?.onBlur}
    />
  );
}

const VariableValueInputFrom = withField(VariableValueInput);

export function VariableModal({
  visible,
  data,
  typeDisabled,
  onCancel,
  onOk,
  variableList,
}: AddVariablesProps) {
  const formApiRef = useRef<FormApi<VariableDef & { value?: string }>>();
  const handleOk = async () => {
    const res = await formApiRef.current?.validate().catch(e => {
      console.error(e);
      Toast.error(I18n.t('prompt_cannot_add_check_form_data'));
    });
    if (res) {
      if (
        (res.type === VariableType.Object || res.type?.includes('array')) &&
        res.value
      ) {
        const v = safeJsonParse(res.value);
        onOk?.(
          { ...res },
          { key: res.key, value: JSON.stringify(v, null, 2) },
          Boolean(data?.key),
        );
        return;
      }
      onOk?.(res, { key: res.key, value: res.value }, Boolean(data?.key));
    }
  };

  const currentData = variableList?.find(it => it.key === data?.key);

  useEffect(() => {
    if (!visible) {
      formApiRef.current?.reset();
    } else {
      if (
        (currentData?.type?.includes('array') ||
          currentData?.type === VariableType.Object) &&
        data?.value
      ) {
        formApiRef.current?.setValues(
          { ...data, ...currentData },
          { isOverride: true },
        );
        const v = safeJsonParse(data.value);

        setTimeout(() => {
          formApiRef.current?.setValue('value', JSON.stringify(v, null, 2));
        }, 100);
      } else {
        if (data) {
          formApiRef.current?.setValues(
            { ...data, ...currentData },
            { isOverride: true },
          );
          setTimeout(() => {
            formApiRef.current?.setValue('value', data.value);
          }, 100);
        }
      }
    }
  }, [visible, currentData, data]);

  return (
    <Modal
      title={
        data?.key
          ? I18n.t('prompt_edit_variable')
          : I18n.t('prompt_add_variable')
      }
      visible={visible}
      onCancel={onCancel}
      size="medium"
      maskClosable={false}
      onOk={handleOk}
      cancelText={I18n.t('cancel')}
      okText={I18n.t('global_btn_confirm')}
    >
      <Form<VariableDef & { value?: string }>
        key={currentData?.key}
        className={styles['variable-modal-form']}
        initValues={{ ...currentData, value: data?.value }}
        getFormApi={api => (formApiRef.current = api)}
        onValueChange={(_values, changeValue) => {
          if (changeValue.type) {
            const newValue =
              changeValue.type === VariableType.Boolean ? 'false' : '';
            formApiRef.current?.setValue('value', newValue);
          }
        }}
        showValidateIcon={false}
        labelPosition="top"
      >
        {({ formState }) => {
          const { type } = formState.values;
          const isJson =
            type?.includes('array') || type === VariableType.Object;
          return (
            <>
              <FormInput
                field="key"
                label={I18n.t('prompt_variable_name')}
                placeholder={I18n.t('prompt_please_input_variable_name')}
                rules={[
                  {
                    required: true,
                    message: I18n.t('prompt_please_input_variable_name'),
                  },
                  {
                    validator: (_, value, callback) => {
                      const regex = new RegExp(
                        `^[a-zA-Z][a-zA-Z0-9_-]{0,${VARIABLE_MAX_LEN}}$`,
                        'gm',
                      );
                      if (value && value.indexOf(' ') === 0) {
                        callback(
                          I18n.t('prompt_variable_name_cannot_start_space'),
                        );
                        return false;
                      }

                      if (value && !regex.test(value)) {
                        callback(I18n.t('prompt_variable_name_format_rule'));
                        return false;
                      }
                      if (
                        variableList?.some(it => it.key === value) &&
                        !data?.key
                      ) {
                        callback(I18n.t('prompt_variable_name_exists'));
                        return false;
                      }
                      return true;
                    },
                  },
                ]}
                disabled={Boolean(data?.key)}
                maxLength={VARIABLE_MAX_LEN}
              />

              <FormSelect
                field="type"
                label={I18n.t('data_type')}
                placeholder={I18n.t('prompt_please_select_variable_data_type')}
                rules={[
                  {
                    required: true,
                    message: I18n.t('prompt_please_select_variable_data_type'),
                  },
                ]}
                optionList={Object.keys(VARIABLE_TYPE_ARRAY_MAP)
                  .filter(
                    key =>
                      key !== VariableType.Placeholder &&
                      key !== VariableType.MultiPart,
                  )
                  .map(key => ({
                    label:
                      VARIABLE_TYPE_ARRAY_MAP[
                        key as keyof typeof VARIABLE_TYPE_ARRAY_MAP
                      ],

                    value: key,
                  }))}
                style={{ width: '100%' }}
                disabled={typeDisabled}
              />

              <VariableValueInputFrom
                field="value"
                label={
                  <div className="flex w-full items-center justify-between">
                    {I18n.t('prompt_variable_value')}
                    {isJson ? (
                      <Typography.Text
                        size="small"
                        className={'!text-[13px]'}
                        link
                        onClick={() => {
                          const json = safeJsonParse(formState.values.value);
                          if (json) {
                            formApiRef.current?.setValue(
                              'value',
                              JSON.stringify(json, null, 2),
                            );
                          }
                        }}
                      >
                        {I18n.t('format_json')}
                      </Typography.Text>
                    ) : null}
                  </div>
                }
                typeValue={type}
                minHeight={26}
                maxHeight={180}
              />
            </>
          );
        }}
      </Form>
    </Modal>
  );
}
