/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable complexity */
/* eslint-disable max-len */

/* eslint-disable @typescript-eslint/no-explicit-any */
import { type RefObject } from 'react';

import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluateSetSelect,
  EvaluateSetVersionSelect,
  OpenDetailText,
} from '@cozeloop/evaluate-components';
import { useBaseURL, useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type EvaluationSet,
  type EvaluationSetVersion,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { IconCozLoading } from '@coze-arch/coze-design/icons';
import { Form, useFormState, withField } from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '@/types/experiment/experiment-create';
import { getEvaluationSetVersion } from '@/request/evaluation-set';

import { evaluateSetValidators } from '../validators/evaluate-set';
import { EvaluateSetColList } from '../../evaluate-set-col-list';

export interface EvaluateSetFormProps {
  formRef: RefObject<Form<CreateExperimentValues>>;
  createExperimentValues: CreateExperimentValues;
  setCreateExperimentValues: React.Dispatch<
    React.SetStateAction<CreateExperimentValues>
  >;
  setNextStepLoading: (loading: boolean) => void;
}

const FormEvaluateSetSelect = withField(EvaluateSetSelect);

export const EvaluateSetForm = (props: EvaluateSetFormProps) => {
  const {
    formRef,
    setNextStepLoading,
    setCreateExperimentValues,
    createExperimentValues,
  } = props;
  const { spaceID } = useSpace();
  const { baseURL } = useBaseURL();
  const formState = useFormState();

  const { values: formValues } = formState;

  const formApi = formRef.current?.formApi;

  const formSetVersionId = formValues?.evaluationSetVersion;

  const formSetId = formValues?.evaluationSet;

  const versionDetail = createExperimentValues?.evaluationSetVersionDetail;

  const versionDetailService = useRequest(
    async (params: { evaluation_set_id: string; version_id: string }) => {
      const evaluationSetVersionDetail = await getEvaluationSetVersion({
        workspace_id: spaceID,
        ...params,
      });
      const mappingData = formApi?.getValue('evalTargetMapping');
      try {
        // 挨个清空 evalTargetMapping 中的 key
        if (mappingData) {
          const mappingKeys = Object.entries(mappingData) || [];
          mappingKeys.forEach(([key, value]) => {
            formApi?.setValue(`evalTargetMapping.${key}` as any, undefined);
          });
        }
        const evaluatorList = formApi?.getValue('evaluatorProList');
        if (evaluatorList?.length) {
          evaluatorList.forEach((item, idx) => {
            const evaluatorMapping = item?.evaluatorMapping;
            // 计算每一项 evaluator 的新 evaluatorMapping
            if (evaluatorMapping && Object.keys(evaluatorMapping).length) {
              const newEvaluatorMapping: any = {};
              Object.entries(evaluatorMapping).forEach(([key, value]) => {
                if (!value?.name || !value?.schemaSourceType) {
                  return;
                }
                // 去除评测集的字段映射
                if (value?.schemaSourceType === 'set') {
                  newEvaluatorMapping[key] = undefined;
                  // 保留评测对象的字段映射
                } else {
                  newEvaluatorMapping[key] = value;
                }
              });
              formApi?.setValue(
                `evaluatorProList.${idx}.evaluatorMapping` as any,
                newEvaluatorMapping,
              );
            }
          });
        }
      } catch (e) {
        console.error('清空 evalTargetMapping 中的 key 失败', e);
      }
      setCreateExperimentValues(prev => {
        const payload = {
          ...prev,
          // 用于渲染的数据, 不在表单上面, 与表单数据有隔离
          evaluationSetVersionDetail:
            evaluationSetVersionDetail.version as EvaluationSetVersion,
          evaluationSetDetail:
            evaluationSetVersionDetail.evaluation_set as EvaluationSet,
        };
        return payload;
      });
    },
    {
      manual: true,
    },
  );

  const renderColumns = (fieldSchemas?: FieldSchema[]) => {
    if (versionDetailService.loading) {
      return (
        <div className="flex flex-row items-center">
          <IconCozLoading className="w-4 h-4 animate-spin coz-fg-secondary" />
          <div className="ml-[6px] text-sm coz-fg-secondary">
            {I18n.t('loading')}
          </div>
        </div>
      );
    }

    return <EvaluateSetColList fieldSchemas={fieldSchemas} />;
  };

  const handleOnEvaluateSetSelectChange = (v: any) => {
    formApi?.setValue('evaluationSetVersion', undefined);
  };

  const handleOnEvaluateSetVersionSelectChange = async (v: any) => {
    if (v && formSetId) {
      setNextStepLoading(true);
      await versionDetailService.runAsync({
        version_id: v,
        evaluation_set_id: formSetId,
      });
      setNextStepLoading(false);
    }
  };

  return (
    <>
      <div className="flex flex-row gap-5 relative">
        <div className="flex-1 w-0">
          <FormEvaluateSetSelect
            className="w-full"
            field="evaluationSet"
            label={I18n.t('evaluation_set')}
            placeholder={I18n.t('please_select', { field: '' })}
            rules={evaluateSetValidators.evaluationSet}
            onChange={handleOnEvaluateSetSelectChange}
            onChangeWithObject={false}
          />
        </div>
        <div className="flex-1 flex flex-row items-end">
          <div className="flex-1 w-0">
            <EvaluateSetVersionSelect
              evaluationSetId={formState?.values?.evaluationSet}
              className="w-full"
              field="evaluationSetVersion"
              label={{
                text: I18n.t('version'),
                className: 'justify-between pr-0',
                extra: (
                  <>
                    {formSetVersionId ? (
                      <OpenDetailText
                        className="absolute top-2.5 right-0"
                        url={`${baseURL}/evaluation/datasets/${formState.values.evaluationSet}?version=${formState.values.evaluationSetVersion}`}
                      />
                    ) : null}
                  </>
                ),
              }}
              placeholder={I18n.t('please_select', { field: '' })}
              rules={evaluateSetValidators.evaluationSetVersion}
              onChange={handleOnEvaluateSetVersionSelectChange}
            />
          </div>
        </div>
      </div>
      <Form.Slot label={I18n.t('description')}>
        <div className="text-sm coz-fg-primary font-normal">
          {versionDetail?.description || '-'}
        </div>
      </Form.Slot>
      <Form.Slot label={I18n.t('column_name')}>
        {formSetVersionId && formSetId
          ? renderColumns(versionDetail?.evaluation_set_schema?.field_schemas)
          : null}
      </Form.Slot>
      <Form.Slot label={I18n.t('data_total_count')}>
        <div className="text-sm coz-fg-primary font-normal">
          {versionDetail?.item_count ?? '-'}
        </div>
      </Form.Slot>
    </>
  );
};
