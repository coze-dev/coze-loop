/* eslint-disable complexity */
import { I18n } from '@cozeloop/i18n-adapter';
import { type EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { Form, Tag } from '@coze-arch/coze-design';

import { EvaluateSetColList } from '@/components/evaluation-set';

import { type PluginEvalTargetFormProps } from '../../../types/evaluate-target';

/**
 * 评测对象, 评测集 选择, 版本选择, 详情, 字段映射
 * @param props
 * @returns
 */
const PluginEvalTargetForm = (props: PluginEvalTargetFormProps) => {
  const { formValues, createExperimentValues } = props;

  const { evalTargetType } = formValues;

  const formSetVersionId = formValues?.evaluationSetVersion;

  const formSetId = formValues?.evaluationSet;

  const versionDetail = createExperimentValues?.evaluationSetVersionDetail;

  const evaluationSetDetail = createExperimentValues?.evaluationSetDetail;

  const fieldSchemas = versionDetail?.evaluation_set_schema?.field_schemas;

  return (
    <>
      {/* 类型存在时才使用 */}
      {evalTargetType === (5 as EvalTargetType) ? (
        <>
          <Form.Slot label={I18n.t('name_and_version')}>
            <span className="text-sm coz-fg-primary font-normal mr-1">
              {evaluationSetDetail?.name || '-'}
            </span>
            <Tag color="primary">{versionDetail?.version || '-'}</Tag>
          </Form.Slot>
          <Form.Slot label={I18n.t('description')}>
            <div className="text-sm coz-fg-primary font-normal">
              {versionDetail?.description || '-'}
            </div>
          </Form.Slot>
          <Form.Slot label={I18n.t('column_name')}>
            {formSetVersionId && formSetId && fieldSchemas ? (
              <EvaluateSetColList fieldSchemas={fieldSchemas} />
            ) : null}
          </Form.Slot>
          <Form.Slot label={I18n.t('data_total_count')}>
            <div className="text-sm coz-fg-primary font-normal">
              {versionDetail?.item_count ?? '-'}
            </div>
          </Form.Slot>
        </>
      ) : null}
    </>
  );
};

export default PluginEvalTargetForm;
