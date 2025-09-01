import { useRef, useState } from 'react';

import { isEqual } from 'lodash-es';
import cs from 'classnames';
import { GuardPoint, Guard } from '@cozeloop/guard';
import { InfoTooltip } from '@cozeloop/components';
import { useNavigateModule, useSpace } from '@cozeloop/biz-hooks-adapter';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import { IconCozDocument } from '@coze-arch/coze-design/icons';
import {
  type FormApi,
  Form,
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
  const formRef = useRef<FormApi<IDatasetCreateForm>>();
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
      Toast.success('创建成功');
      navigate(`evaluation/datasets/${res.evaluation_set_id}`);
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
            如何创建评测集
          </Typography.Text>
        </div>
      </div>
      <Form<IDatasetCreateForm>
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
            <FormSectionLayout title="基本信息" className="!mb-[14px]">
              <FormInput
                label="名称"
                maxLength={50}
                field="name"
                placeholder="请输入评测集名称"
                rules={[
                  { required: true, message: '请输入评测集名称' },
                  { validator: sourceNameRuleValidator },
                ]}
              ></FormInput>
              <FormTextArea
                label="描述"
                field="description"
                placeholder="请输入评测集描述"
                maxLength={200}
                maxCount={200}
              ></FormTextArea>
            </FormSectionLayout>

            <FormSectionLayout
              title={
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1">
                    配置列
                    <InfoTooltip content="评测集创建完成后，仍可修改列配置" />
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
                            title: '信息未保存',
                            width: 420,
                            content: '切换后当前修改会被覆盖',
                            onOk: () => {
                              setTemplate(newValue as CreateTemplate);
                              formApi.setValue(
                                'columns',
                                COLUMNS_MAP[newValue as CreateTemplate],
                              );
                            },
                            okText: '确认',
                            okButtonColor: 'yellow',
                            cancelText: '取消',
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
      </Form>
      <div className="flex justify-end w-[800px] m-[24px] mx-auto">
        <Guard point={GuardPoint['eval.dataset_create.create']}>
          <Button
            color="hgltplus"
            onClick={() => {
              formRef.current?.submitForm();
            }}
            loading={loading}
          >
            创建
          </Button>
        </Guard>
      </div>
    </div>
  );
};
