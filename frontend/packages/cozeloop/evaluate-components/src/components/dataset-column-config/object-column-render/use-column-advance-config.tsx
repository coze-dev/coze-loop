// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { Fragment, useEffect, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
  resetAdditionalProperty,
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
  disableChangeDatasetType: boolean;
}
const FormRemovePropertyField = withField(RemovePropertyField);

export const useColumnAdvanceConfig = ({
  fieldKey,
  disableChangeDatasetType,
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
          title: I18n.t('cozeloop_open_evaluate_confirm_switch'),
          content: I18n.t('evaluation_set_json_schema_invalid_tips'),
          onOk: () => {
            fieldApi.setValue({
              ...fieldValue,
              children: [],
              inputType: newType,
            });
            setInputType(newType as InputType);
          },
          okButtonColor: 'yellow',
          okText: I18n.t('global_btn_confirm'),
          cancelText: I18n.t('cancel'),
        });
      }
    }
  };
  const isObject =
    fieldValue?.type === DataType.Object ||
    fieldValue?.type === DataType.ArrayObject;

  const advanceRules = [
    {
      label: I18n.t('redundant_field_check'),
      hideen: !isObject,
      tooltip: I18n.t(
        'cozeloop_open_evaluate_enable_object_type_validation_rules',
      ),
      node:
        showAdditional && !disableChangeDatasetType ? (
          <Popconfirm
            title={I18n.t(
              'cozeloop_open_evaluate_confirm_disable_redundant_field_validation',
            )}
            content={I18n.t(
              'cozeloop_open_evaluate_disable_redundant_validation_default_no',
            )}
            okText={I18n.t('global_btn_confirm')}
            cancelText={I18n.t('cancel')}
            okButtonColor="yellow"
            zIndex={10000}
            onConfirm={() => {
              const newFieldSchema = resetAdditionalProperty(fieldValue);
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
            disabled={disableChangeDatasetType}
            onChange={checked => {
              setShowAdditional(checked);
            }}
          />
        ),
    },
  ];

  const menuItems = [
    {
      title: I18n.t('advanced_validation_rule'),
      hideen: !isObject || isJSON,
      children: advanceRules,
    },
    {
      title: I18n.t('data_processing_short'),
      hideen: !isObject,
      tooltip: I18n.t(
        'cozeloop_open_evaluate_data_processing_after_validation_import',
      ),
      children: [
        {
          label: I18n.t('remove_redundant_fields'),
          tooltip: I18n.t(
            'cozeloop_open_evaluate_remove_fields_outside_structure_on_import',
          ),
          node: (
            <FormRemovePropertyField
              disabled={disableChangeDatasetType}
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
              <Select.Option value={InputType.Form}>
                {I18n.t('visual_configuration')}
              </Select.Option>
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
          disabled={disableChangeDatasetType}
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
