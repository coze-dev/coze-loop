// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { useRef, useState } from 'react';

import { isEqual } from 'lodash-es';
import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { GuardPoint, Guard } from '@cozeloop/guard';
import {
  InfoTooltip,
  SentinelForm,
  type SentinelFormApi,
} from '@cozeloop/components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozDocument } from '@coze-arch/coze-design/icons';
import {
  Button,
  Toast,
  FormInput,
  FormTextArea,
  Typography,
  Modal,
} from '@coze-arch/coze-design';

import { useGlobalEvalConfig } from '@/stores/eval-global-config';

import { DatasetColumnConfig } from '../dataset-column-config';
import { sourceNameRuleValidator } from '../../utils/source-name-rule';
import { convertDataTypeToSchema } from '../../utils/field-convert';
import {
  COLUMNS_MAP,
  CreateTemplate,
  DEFAULT_DATASET_CREATE_FORM,
  type IDatasetCreateForm,
} from './type';
import { FormSectionLayout } from './form-section-layout';
import { CreateDatasetTemplate } from './create-template';

import styles from './index.module.less';
export interface DatasetCreateFormProps {
  header?: React.ReactNode;
}

// const FormColumnConfig = withField()

export const DatasetCreateForm = ({ header }: DatasetCreateFormProps) => {
  const formRef = useRef<SentinelFormApi<IDatasetCreateForm>>();
  const { spaceID } = useSpace();
  const navigate = useNavigateModule();
  const [template, setTemplate] = useState<CreateTemplate>(
    CreateTemplate.Default,
  );
  const config = useGlobalEvalConfig();
  const [loading, setLoading] = useState(false);
  const onSubmit = async (values: IDatasetCreateForm) => {
    try {
      setLoading(true);
      const res = await StoneEvaluationApi.CreateEvaluationSet({
        name: values.name,
        workspace_id: spaceID,
        description: values.description,
        evaluation_set_schema: {
          field_schemas:
            values.columns?.map(item => convertDataTypeToSchema(item)) || [],
          workspace_id: spaceID,
        },
      });
      Toast.success(I18n.t('create_success'));
      formRef.current?.submitLog?.();
      navigate(`evaluation/datasets/${res.evaluation_set_id}`);
    } catch (e) {
      console.log(e);
      formRef.current?.submitLog?.(true, e);
    } finally {
      setLoading(false);
    }
  };
  return (
    <div className="flex h-full flex-col">
      <div className="flex justify-between px-6 pt-[12px] py-3 h-[56px] box-border text-[18px]">
        {header}
        <div className="flex items-center gap-[2px]">
          <IconCozDocument className="coz-fg-secondary" />
          <Typography.Text
            className="cursor-pointer !coz-fg-secondary"
            onClick={() => {
              window.open(
                'https://loop.coze.cn/open/docs/cozeloop/create-dataset',
                '_blank',
              );
            }}
          >
            {I18n.t('how_to_create_a_review_set')}
          </Typography.Text>
        </div>
      </div>
      <SentinelForm<IDatasetCreateForm>
        formID={I18n.t('evaluate_evaluation_new_testset')}
        getFormApi={formApi => {
          formRef.current = formApi;
        }}
        initValues={DEFAULT_DATASET_CREATE_FORM}
        className={cs(styles.form, 'styled-scrollbar')}
        onSubmit={onSubmit}
        onValueChange={values => {
          console.log('values', values);
        }}
      >
        {({ formApi, formState }) => (
          <div className="w-[800px] mx-auto flex flex-col gap-[40px]">
            <FormSectionLayout
              title={I18n.t('basic_information')}
              className="!mb-[14px]"
            >
              <FormInput
                label={I18n.t('name')}
                maxLength={50}
                field="name"
                placeholder={I18n.t('enter_evaluation_name')}
                rules={[
                  { required: true, message: I18n.t('enter_evaluation_name') },
                  { validator: sourceNameRuleValidator },
                ]}
              ></FormInput>
              <FormTextArea
                label={I18n.t('description')}
                field="description"
                placeholder={I18n.t('please_enter_a_review_set_description')}
                maxLength={200}
                maxCount={200}
              ></FormTextArea>
            </FormSectionLayout>

            <FormSectionLayout
              title={
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1">
                    {I18n.t('configuration_column')}
                    <InfoTooltip content={I18n.t('can_still_modify_column')} />
                  </div>
                  {config.showCreateEvaluateSetTemplateSelect ? (
                    <CreateDatasetTemplate
                      onChange={newValue => {
                        const columnsValue = formState?.values?.columns;
                        const oldTemplate = COLUMNS_MAP[template];
                        if (isEqual(columnsValue, oldTemplate)) {
                          setTemplate(newValue as CreateTemplate);
                          formApi.setValue(
                            'columns',
                            COLUMNS_MAP[newValue as CreateTemplate],
                          );
                        } else {
                          Modal.warning({
                            title: I18n.t('information_unsaved'),
                            width: 420,
                            content: I18n.t(
                              'switching_modification_overwritten_tips',
                            ),
                            onOk: () => {
                              setTemplate(newValue as CreateTemplate);
                              formApi.setValue(
                                'columns',
                                COLUMNS_MAP[newValue as CreateTemplate],
                              );
                            },
                            okText: I18n.t('global_btn_confirm'),
                            okButtonColor: 'yellow',
                            cancelText: I18n.t('cancel'),
                          });
                        }
                      }}
                    ></CreateDatasetTemplate>
                  ) : null}
                </div>
              }
              className="!mb-[24px]"
            >
              <DatasetColumnConfig
                key={template}
                fieldKey="columns"
                showAddButton
              ></DatasetColumnConfig>
            </FormSectionLayout>
          </div>
        )}
      </SentinelForm>
      <div className="flex justify-end w-[800px] m-[24px] mx-auto">
        <Guard point={GuardPoint['eval.dataset_create.create']}>
          <Button
            color="hgltplus"
            onClick={() => {
              formRef.current?.submitForm();
            }}
            loading={loading}
          >
            {I18n.t('create')}
          </Button>
        </Guard>
      </div>
    </div>
  );
};
