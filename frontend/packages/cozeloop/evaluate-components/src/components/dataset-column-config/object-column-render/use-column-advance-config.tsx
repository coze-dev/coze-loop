/* eslint-disable @coze-arch/max-line-per-function */
import { Fragment, useEffect, useState } from 'react';

import { IconCozSetting } from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Dropdown,
  Modal,
  Popconfirm,
  Select,
  Switch,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

import {
  convertFieldObjectToSchema,
  validColumnSchema,
  convertJSONSchemaToFieldObject,
} from '@/utils/jsonschema-convert';
import {
  getColumnHasRequiredAndAdditional,
  resetAdditonalProperty,
} from '@/utils/field-convert';
import {
  DataType,
  InputType,
  type ConvertFieldSchema,
} from '@/components/dataset-item/type';
import { InfoIconTooltip } from '@/components/common';

import { RemovePropertyField } from './remove-property-field';
interface Props {
  fieldKey: string;
  disabelChangeDatasetType: boolean;
}
const FormRemovePropertyField = withField(RemovePropertyField);

export const useColumnAdvanceConfig = ({
  fieldKey,
  disabelChangeDatasetType,
}: Props) => {
  const fieldApi = useFieldApi(fieldKey);
  const transFieldApi = useFieldApi(`${fieldKey}.default_transformations`);
  const fieldValue = fieldApi?.getValue() as ConvertFieldSchema;
  // object输入方式
  const [inputType, setInputType] = useState<InputType>(
    fieldValue?.inputType || InputType.Form,
  );

  useEffect(() => {
    if (fieldValue?.inputType) {
      setInputType(fieldValue?.inputType || InputType.Form);
    }
  }, [fieldValue?.inputType]);
  const { hasAdditionalProperties } =
    getColumnHasRequiredAndAdditional(fieldValue);
  const [showAdditional, setShowAdditional] = useState<boolean>(
    hasAdditionalProperties,
  );
  const isForm = inputType === InputType.Form;
  const isJSON = inputType === InputType.JSON;
  const onInputTypeChange = newType => {
    if (newType === InputType.JSON) {
      const schema = convertFieldObjectToSchema({
        type: fieldValue.type,
        key: '',
        additionalProperties: fieldValue.additionalProperties,
        children: fieldValue.children,
      });
      fieldApi.setValue({
        ...fieldValue,
        schema: JSON.stringify(schema, null, 2),
        inputType: newType,
      });
      setInputType(newType as InputType);
    }
    if (newType === InputType.Form) {
      const isValid = validColumnSchema({
        schema: fieldValue?.schema || '',
        type: fieldValue.type,
      });
      if (isValid) {
        try {
          const objectSchema = convertJSONSchemaToFieldObject(
            JSON.parse(fieldValue?.schema || ''),
          );
          const newFieldValue = {
            ...fieldValue,
            children: objectSchema?.children || [],
            additionalProperties: objectSchema?.additionalProperties,
            inputType: newType,
          };
          const { hasAdditionalProperties: newHasAdditionalProperties } =
            getColumnHasRequiredAndAdditional(newFieldValue);
          setShowAdditional(showAdditional || newHasAdditionalProperties);
          fieldApi.setValue(newFieldValue);
        } catch (error) {
          console.error('error', error);
        }
        setInputType(newType as InputType);
      } else {
        Modal.confirm({
          title: '确认切换？',
          content: '当前JSON Schema不合法，切换会导致配置丢失，是否继续切换',
          onOk: () => {
            fieldApi.setValue({
              ...fieldValue,
              children: [],
              inputType: newType,
            });
            setInputType(newType as InputType);
          },
          okButtonColor: 'yellow',
          okText: '确认',
          cancelText: '取消',
        });
      }
    }
  };
  const isObject =
    fieldValue?.type === DataType.Object ||
    fieldValue?.type === DataType.ArrayObject;

  const advanceRules = [
    {
      label: '冗余字段校验',
      hideen: !isObject,
      tooltip:
        '开启后，Object数据类型支持配置校验规则，用于控制数据导入时，如果存在Object数据结构定义之外的字段，该数据是否准入',
      node:
        showAdditional && !disabelChangeDatasetType ? (
          <Popconfirm
            title="是否关闭 冗余字段校验 配置项"
            content="关闭后 冗余字段校验 配置将采用默认配置“否”，确定关闭吗？"
            okText="确认"
            cancelText="取消"
            okButtonColor="yellow"
            zIndex={10000}
            onConfirm={() => {
              const newFieldSchema = resetAdditonalProperty(fieldValue);
              fieldApi.setValue(newFieldSchema);
              setShowAdditional(false);
            }}
          >
            <div>
              <Switch checked={showAdditional} size="small" />
            </div>
          </Popconfirm>
        ) : (
          <Switch
            checked={showAdditional}
            size="small"
            disabled={disabelChangeDatasetType}
            onChange={checked => {
              setShowAdditional(checked);
            }}
          />
        ),
    },
  ];
  const menuItems = [
    {
      title: '高级校验规则',
      hideen: !isObject || isJSON,
      children: advanceRules,
    },
    {
      title: '数据加工',
      hideen: !isObject,
      tooltip: '导入数据时，在完成校验后对数据的加工操作。',
      children: [
        {
          label: '移除冗余字段',
          tooltip: '导入数据时，是否移除数据结构定义之外字段',
          node: (
            <FormRemovePropertyField
              disabled={disabelChangeDatasetType}
              initValue={fieldValue?.default_transformations}
              fieldClassName="!py-0"
              onChange={value => {
                transFieldApi.setValue(value);
              }}
              noLabel
              className="w-full "
              field={`${fieldKey}.temp_default_transformations`}
            />
          ),
        },
      ],
    },
  ];

  const AdvanceConfigNode = (
    <>
      <div className="flex items-center gap-2  relative">
        {isObject ? (
          <>
            <Select
              value={inputType}
              className="semi-select-small !h-[24px] !min-h-[24px]"
              size="small"
              onChange={onInputTypeChange}
            >
              <Select.Option value={InputType.Form}>可视化配置</Select.Option>
              <Select.Option value={InputType.JSON}>JSON</Select.Option>
            </Select>
            <Divider layout="vertical" className="w-[1px] h-[14px]" />
          </>
        ) : null}

        <Dropdown
          trigger="click"
          keepDOM
          render={
            <Dropdown.Menu className="!p-3 !pt-2 flex flex-col gap-[10px] ">
              {menuItems.map((item, index) =>
                item.hideen ? null : (
                  <Fragment key={item.title}>
                    <div className="coz-fg-secondary font-semibold text-[12px]">
                      {item.title}
                    </div>
                    {item.children.map(child =>
                      child.hideen ? null : (
                        <div className="flex w-[160px] items-center justify-between">
                          <div className="flex gap-1">
                            {child.label}
                            <InfoIconTooltip
                              tooltip={child.tooltip}
                            ></InfoIconTooltip>
                          </div>
                          {child.node}
                        </div>
                      ),
                    )}
                    {index === 0 ? (
                      <Divider
                        className="w-[160px] h-[1px]"
                        layout="horizontal"
                      />
                    ) : null}
                  </Fragment>
                ),
              )}
            </Dropdown.Menu>
          }
        >
          <Button
            color="secondary"
            size="mini"
            className={!isObject ? '!hidden' : ''}
            icon={<IconCozSetting className="w-[14px] h-[14px]" />}
          ></Button>
        </Dropdown>
      </div>
      {isObject ? (
        <FormRemovePropertyField
          disabled={disabelChangeDatasetType}
          noLabel
          className="hidden"
          field={`${fieldKey}.default_transformations`}
        />
      ) : null}
    </>
  );
  return {
    AdvanceConfigNode,
    showAdditional,
    isForm,
    isJSON,
    inputType,
    isObject,
  };
};
