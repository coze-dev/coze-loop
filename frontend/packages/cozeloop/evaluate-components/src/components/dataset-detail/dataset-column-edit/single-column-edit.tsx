// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef, useState } from 'react';

import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, useGuard } from '@cozeloop/guard';
import { EditIconButton } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSet,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Divider,
  Form,
  type FormApi,
  FormInput,
  Modal,
  Typography,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

interface ColumnForm {
  columns: FieldSchema[];
}
import { DataType } from '@/components/dataset-item/type';
import { useColumnAdvanceConfig } from '@/components/dataset-column-config/object-column-render/use-column-advance-config';
import { AdditionalPropertyField } from '@/components/dataset-column-config/object-column-render/additional-property-field';
import { MultipartRender } from '@/components/dataset-column-config/multipart-column-render';

import { RequiredField } from '../../dataset-column-config/object-column-render/required-field';
import { ObjectStructRender } from '../../dataset-column-config/object-column-render/object-struct-render';
import { DataTypeSelect } from '../../dataset-column-config/field-type';
import { columnNameRuleValidator } from '../../../utils/source-name-rule';
import {
  convertDataTypeToSchema,
  convertSchemaToDataType,
} from '../../../utils/field-convert';

import { createPortal } from 'react-dom';

const FormDataTypeSelect = withField(DataTypeSelect);
const FormRequiredField = withField(RequiredField);
const FormAdditionalPropertyField = withField(AdditionalPropertyField);

export const DatasetSingleColumnEdit = ({
  datasetDetail,
  onRefresh,
  currentField,
  disabledDataTypeSelect,
}: {
  datasetDetail?: EvaluationSet;
  onRefresh: () => void;
  currentField: FieldSchema;
  disabledDataTypeSelect?: boolean;
}) => {
  const formApiRef = useRef<FormApi>();
  const { spaceID } = useSpace();
  const [visible, setVisible] = useState(false);
  const [loading, setLoading] = useState(false);

  const { data: guardData } = useGuard({
    point: GuardPoint['eval.dataset.edit_col'],
  });

  const handleSubmit = async (values: ColumnForm) => {
    try {
      setLoading(true);
      const columns = values?.columns?.map(item =>
        convertDataTypeToSchema(item),
      );
      await StoneEvaluationApi.UpdateEvaluationSetSchema({
        evaluation_set_id: datasetDetail?.id as string,
        fields: columns,
        workspace_id: spaceID,
      });
      onRefresh();
      setVisible(false);
    } catch (error) {
      console.error(error);
    }
    setLoading(false);
  };
  const fieldSchemas =
    datasetDetail?.evaluation_set_version?.evaluation_set_schema?.field_schemas;
  const initColumnsData =
    fieldSchemas?.map(item => convertSchemaToDataType(item)) || [];
  const selectedFieldIndex = fieldSchemas?.findIndex(
    item => item.key === currentField?.key,
  );
  if (selectedFieldIndex === -1 || selectedFieldIndex === undefined) {
    return <></>;
  }
  const protalID = `column-edit-modal-${selectedFieldIndex}`;
  return (
    <>
      <EditIconButton
        onClick={() => {
          setVisible(true);
        }}
      />
      <Modal
        visible={visible}
        width={960}
        title={
          <div className="flex overflow-hidden w-full justify-between items-center">
            <div className="flex">
              <span>{I18n.t('cozeloop_open_evaluate_edit_column')}</span>
              <Typography.Text
                className="!text-[18px] !font-semibold flex-1"
                ellipsis={{
                  showTooltip: { opts: { theme: 'dark', zIndex: 1900 } },
                }}
              >
                {currentField?.name}
              </Typography.Text>
            </div>
            <div id={protalID}></div>
          </div>
        }
        onCancel={() => {
          setVisible(false);
        }}
        onOk={() => {
          formApiRef.current?.submitForm();
        }}
        keepDOM={false}
        okText={I18n.t('save')}
        okButtonProps={{ loading, disabled: guardData.readonly }}
        cancelText={I18n.t('cancel')}
        zIndex={1000}
      >
        <Form<ColumnForm>
          getFormApi={formApi => (formApiRef.current = formApi)}
          onSubmit={handleSubmit}
          className="pb-4"
          initValues={{
            columns: initColumnsData,
          }}
        >
          {({ formState, formApi }) => (
            <>
              <div className="flex gap-2 flex-wrap">
                <FormInput
                  label={I18n.t('name')}
                  maxLength={50}
                  fieldClassName="flex-1"
                  field={`columns.${selectedFieldIndex}.name`}
                  rules={[
                    {
                      required: true,
                      message: I18n.t('please_enter_column_name'),
                    },
                    {
                      validator: columnNameRuleValidator,
                    },
                    {
                      validator: (_, value) => {
                        if (
                          fieldSchemas
                            ?.filter(
                              (data, dataIndex) =>
                                dataIndex !== selectedFieldIndex,
                            )
                            .some(item => item.name === value)
                        ) {
                          return false;
                        }
                        return true;
                      },
                      message: I18n.t('column_name_exists'),
                    },
                  ]}
                ></FormInput>

                <ObjectContent
                  fieldKey={`columns.${selectedFieldIndex}`}
                  disabelChangeDatasetType={disabledDataTypeSelect}
                  protalID={protalID}
                />
              </div>
            </>
          )}
        </Form>
      </Modal>
    </>
  );
};

export const ObjectContent = ({
  fieldKey,
  disabelChangeDatasetType = false,
  protalID,
}: {
  fieldKey: string;
  disabelChangeDatasetType?: boolean;
  protalID: string;
}) => {
  const fieldApi = useFieldApi(fieldKey);
  const {
    AdvanceConfigNode,
    showAdditional,
    inputType,
    isForm,
    isObject,
    isJSON,
  } = useColumnAdvanceConfig({
    fieldKey,
    disabelChangeDatasetType,
  });
  return (
    <>
      <FormDataTypeSelect
        label={I18n.t('data_type')}
        labelWidth={90}
        zIndex={1070}
        fieldClassName="w-[190px]"
        disabled={disabelChangeDatasetType || isJSON}
        onChange={newType => {
          fieldApi.setValue({
            ...fieldApi.getValue(),
            children: [],
            schema: '',
            additionalProperties: true,
          });
        }}
        field={`${fieldKey}.type`}
        className="w-full"
        rules={[{ required: true, message: I18n.t('select_data_type') }]}
      ></FormDataTypeSelect>
      <FormRequiredField
        label={{
          text: I18n.t('required'),
          required: true,
        }}
        fieldClassName="w-[60px]"
        className="w-full"
        disabled={disabelChangeDatasetType}
        field={`${fieldKey}.isRequired`}
      />
      <FormAdditionalPropertyField
        disabled={disabelChangeDatasetType}
        label={{
          text: I18n.t('redundant_fields_allowed'),
          required: true,
        }}
        fieldClassName={cs(
          'w-[120px]',
          isObject && isForm && showAdditional ? '' : 'hidden',
        )}
        className="w-full"
        field={`${fieldKey}.additionalProperties`}
      />
      <Form.TextArea
        label={I18n.t('description')}
        maxCount={200}
        autosize={{ minRows: 1, maxRows: 6 }}
        fieldClassName="w-full"
        field={`${fieldKey}.description`}
      ></Form.TextArea>
      <div className="w-full">
        {createPortal(
          <div className="flex gap-1 items-center">
            {AdvanceConfigNode}
            <Divider layout="vertical" className="w-[1px] mr-1 h-[14px]" />
          </div>,
          document.getElementById(protalID) || document.body,
        )}
        {fieldApi.getValue()?.type === DataType.MultiPart ? (
          <MultipartRender inputType={inputType} />
        ) : null}
        {isObject ? (
          <ObjectStructRender
            inputType={inputType}
            showAdditional={showAdditional}
            fieldKey={fieldKey}
            disabelChangeDatasetType={disabelChangeDatasetType || false}
          />
        ) : null}
      </div>
    </>
  );
};
