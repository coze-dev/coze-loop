import { I18n } from '@cozeloop/i18n-adapter';
import {
  type EvalTargetDefinition,
  useEvalTargetDefinition,
  DEFAULT_TEXT_STRING_SCHEMA,
} from '@cozeloop/evaluate-components';
import { useBaseURL } from '@cozeloop/biz-hooks-adapter';
import { type EvaluationSetVersion } from '@cozeloop/api-schema/evaluation';
import { Tag, useFormState } from '@coze-arch/coze-design';

import { type CreateExperimentValues } from '@/types/experiment/experiment-create';
import { ReadonlyMappingItem } from '@/components/mapping-item-field/readonly-mapping-item';

import { OpenDetailButton } from '../../open-detail-button';
import { EvaluateSetColList } from '../../evaluate-set-col-list';
import { EvaluateItemRender } from '../../evaluate-item-render';

export interface ViewSubmitFormProps {
  initialValues?: CreateExperimentValues;
  onChange?: (values: Partial<CreateExperimentValues>) => void;
  onSubmit?: () => void;
  startTime?: number;
  isCopyExperiment?: boolean;
  currentEvaluator?: EvalTargetDefinition;
}

// 渲染评测对象部分
const RenderEvalTarget = ({
  renderValues,
  formValues,
  currentEvalTargetDefinition,
}: {
  renderValues: CreateExperimentValues;
  formValues: CreateExperimentValues;
  currentEvalTargetDefinition?: EvalTargetDefinition;
}) => {
  // 没有选择评测对象, 就是使用了评测集作为评测对象, 返回提示(公共逻辑
  if (!formValues?.evalTargetType) {
    return (
      <>
        <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
          {I18n.t('evaluation_object')}
        </div>
        <div className="text-[14px] font-normal coz-fg-primary">
          {I18n.t('this_step_skipped')}
        </div>
        <div className="h-10" />
      </>
    );
  }

  const { evalTargetView: EvalTargetView } = (currentEvalTargetDefinition ||
    {}) as EvalTargetDefinition;

  return EvalTargetView ? (
    <EvalTargetView values={renderValues} formValues={formValues} />
  ) : null;
};

// 渲染基础信息部分
const RenderBasicInfo = ({ name, desc }: { name?: string; desc?: string }) => (
  <>
    <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
      {I18n.t('basic_info')}
    </div>
    <div className="flex flex-row gap-5">
      <div className="flex-1 w-0">
        <div className="text-sm font-medium coz-fg-primary mb-2">
          {I18n.t('name')}
        </div>
        <div className="text-sm font-normal coz-fg-primary">{name || '-'}</div>
      </div>
      <div className="flex-1 w-0">
        <div className="text-sm font-medium coz-fg-primary mb-2">
          {I18n.t('description')}
        </div>
        <div className="text-sm font-normal coz-fg-primary">{desc || '-'}</div>
      </div>
    </div>
  </>
);

interface EvaluationSetInfo {
  name?: string;
  id?: string | number;
}

// 渲染评测集部分
const RenderEvaluationSet = ({
  evaluationSetDetail,
  evaluationSetVersionDetail,
  baseURL,
}: {
  evaluationSetDetail?: EvaluationSetInfo;
  evaluationSetVersionDetail?: EvaluationSetVersion;
  baseURL: string;
}) => (
  <>
    <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
      {I18n.t('evaluation_set')}
    </div>
    <div className="flex flex-row gap-5">
      <div className="flex-1 w-0">
        <div className="text-sm font-medium coz-fg-primary mb-2">
          {I18n.t('name_and_version')}
        </div>
        <div className="flex flex-row items-center gap-1">
          <div className="text-sm font-normal coz-fg-primary">
            {evaluationSetDetail?.name || '-'}
          </div>
          <Tag
            color="primary"
            className="!h-5 !px-2 !py-[2px] rounded-[3px] min-w-[32px]"
          >
            {evaluationSetVersionDetail?.version || '-'}
          </Tag>
          <OpenDetailButton
            url={`${baseURL}/evaluation/datasets/${evaluationSetDetail?.id}?version=${evaluationSetVersionDetail?.id}`}
          />
        </div>
      </div>
      <div className="flex-1 w-0">
        <div className="text-sm font-medium coz-fg-primary mb-2">
          {I18n.t('column_name')}
        </div>
        <EvaluateSetColList
          fieldSchemas={
            evaluationSetVersionDetail?.evaluation_set_schema?.field_schemas
          }
        />
      </div>
    </div>
  </>
);

export const ViewSubmitForm = (props: {
  createExperimentValues: CreateExperimentValues;
}) => {
  const { createExperimentValues } = props;
  const { baseURL } = useBaseURL();
  const formState = useFormState();
  const formValues = formState.values as CreateExperimentValues;

  const values = formValues || {};

  const { evaluationSetDetail, evaluationSetVersionDetail } =
    createExperimentValues;

  const { evalTargetType, evalTargetMapping, evaluatorProList } =
    formValues || {};

  const { getEvalTargetDefinition } = useEvalTargetDefinition();

  const currentEvalTargetDefinition = getEvalTargetDefinition(
    evalTargetType as number,
  );

  const { viewSubmitFieldMappingPreview: ViewSubmitFieldMappingPreview } =
    (currentEvalTargetDefinition || {}) as EvalTargetDefinition;

  return (
    <div className="flex flex-col pt-3">
      <RenderBasicInfo name={values.name} desc={values.desc} />

      <div className="h-10" />

      <RenderEvaluationSet
        evaluationSetDetail={evaluationSetDetail}
        evaluationSetVersionDetail={evaluationSetVersionDetail}
        baseURL={baseURL}
      />

      <div className="h-10" />

      {createExperimentValues.evalTargetType ? (
        <RenderEvalTarget
          renderValues={createExperimentValues}
          currentEvalTargetDefinition={currentEvalTargetDefinition}
          formValues={formValues}
        />
      ) : (
        <div>
          <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
            {I18n.t('evaluation_object')}
          </div>
          <div className="coz-fg-primary">
            {I18n.t('this_step_skipped_period')}
          </div>
          <div className="h-10" />
        </div>
      )}

      {evalTargetMapping ? (
        <div>
          <div className="text-sm font-medium coz-fg-primary mb-2">
            {I18n.t('field_mapping')}
          </div>
          {ViewSubmitFieldMappingPreview ? (
            <ViewSubmitFieldMappingPreview
              createExperimentValues={createExperimentValues}
            />
          ) : (
            <div className="flex flex-col gap-3">
              {Object.entries(evalTargetMapping || {}).map(([k, v]) => (
                <ReadonlyMappingItem
                  key={k}
                  keyTitle={I18n.t('evaluation_object')}
                  keySchema={{
                    name: k,
                    ...DEFAULT_TEXT_STRING_SCHEMA,
                  }}
                  optionSchema={v}
                />
              ))}
            </div>
          )}
          <div className="h-10" />
        </div>
      ) : null}

      <div className="text-[16px] leading-[22px] font-medium coz-fg-primary mb-5">
        {I18n.t('evaluator')}
      </div>

      <div className="flex flex-col gap-5">
        {evaluatorProList?.filter(item => item.evaluator).length ? (
          evaluatorProList.map((evaluatorPro, index) => (
            <EvaluateItemRender key={index} evaluatorPro={evaluatorPro} />
          ))
        ) : (
          <div className="coz-fg-primary">
            {I18n.t('this_step_skipped_period')}
          </div>
        )}
      </div>
    </div>
  );
};
