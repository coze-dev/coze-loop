/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @typescript-eslint/no-magic-numbers */
/* eslint-disable security/detect-non-literal-regexp */

import { useEffect, useRef } from 'react';

import classNames from 'classnames';
import Ajv from 'ajv';
import { safeParseJson } from '@cozeloop/toolkit';
import { BaseJsonEditor } from '@cozeloop/prompt-components';
import { formateDecimalPlacesString } from '@cozeloop/components';
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
    return '输入内容不符合列的字段定义';
  }
  const errorInfo = errors?.[0];
  const type = errorInfo?.keyword;
  const instancePath = errorInfo?.instancePath;
  switch (type) {
    case 'type': {
      return `${instancePath}数据类型不符合字段定义`;
    }
    case 'required': {
      return `缺少必填字段"${instancePath ? `${instancePath}/` : ''}${errorInfo?.params?.missingProperty}"`;
    }
    case 'additionalProperties': {
      return `存在冗余字段${errorInfo?.params?.additionalProperty}`;
    }
    default: {
      return '输入内容不符合列的字段定义';
    }
  }
};

const ajv = new Ajv();

const validateJson = (value: string, typeValue: VariableType) => {
  const data = safeParseJson(value);
  if (typeof data !== 'object') {
    return '输入内容不是合法json格式';
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
            ? '请输入整数'
            : '请输入浮点数，最多保留4位小数'
        }
        style={{ width: '100%' }}
        value={value}
        onChange={v => {
          // 使用正则表达式检查是否为有效数字（不包括科学记数法）
          const isValidNumber = /^-?\d*\.?\d*$/.test(`${v}`);
          if (!isValidNumber) {
            formApi?.setError('value', '输入内容不符合列的字段定义'); // 设置错误信息
          } else {
            formApi?.setError('value', ''); // 清除错误信息
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
        <BaseJsonEditor
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
      placeholder="请输入变量值"
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
      Toast.error('无法新增，请检查表单数据');
    });
    if (res) {
      if (
        (res.type === VariableType.Object || res.type?.includes('array')) &&
        res.value
      ) {
        const v = safeParseJson(res.value);
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
        const v = safeParseJson(data.value);

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
      title={data?.key ? '编辑变量' : '新增变量'}
      visible={visible}
      onCancel={onCancel}
      size="medium"
      maskClosable={false}
      onOk={handleOk}
      cancelText="取消"
      okText="确认"
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
                label="变量名称"
                placeholder="请输入变量名称"
                rules={[
                  { required: true, message: '请输入变量名称' },
                  {
                    validator: (_, value) => {
                      const regex = new RegExp(
                        `^[a-zA-Z][a-zA-Z0-9_-]{0,${VARIABLE_MAX_LEN}}$`,
                        'gm',
                      );
                      if (value && value.indexOf(' ') === 0) {
                        return new Error('变量名不能以空格开头');
                      }

                      if (value && !regex.test(value)) {
                        return new Error(
                          '变量名格式仅支持字母、数字、下划线、中划线，且不能以数字开头',
                        );
                      }
                      if (
                        variableList?.some(it => it.key === value) &&
                        !data?.key
                      ) {
                        return new Error('变量名已存在');
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
                label="数据类型"
                placeholder="请选择变量的数据类型"
                rules={[{ required: true, message: '请选择变量的数据类型' }]}
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
                    变量值
                    {isJson ? (
                      <Typography.Text
                        size="small"
                        className={'!text-[13px]'}
                        link
                        onClick={() => {
                          const json = safeParseJson(formState.values.value);
                          if (json) {
                            formApiRef.current?.setValue(
                              'value',
                              JSON.stringify(json, null, 2),
                            );
                          }
                        }}
                      >
                        格式化JSON
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
