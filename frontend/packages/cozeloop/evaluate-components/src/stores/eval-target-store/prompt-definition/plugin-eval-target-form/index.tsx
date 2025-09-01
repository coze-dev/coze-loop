import { useEffect, useMemo } from 'react';

import { isEmpty } from 'lodash-es';
import { EvalTargetType } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { Form, Tooltip } from '@coze-arch/coze-design';

import { type PluginEvalTargetFormProps } from '@/types/evaluate-target';
import { DEFAULT_TEXT_STRING_SCHEMA } from '@/const/evaluate-target';
import { EvaluateTargetMappingField } from '@/components/selectors/evaluate-target';

import PromptEvalTargetVersionFormSelect from '../../components/eval-target-prompt-version-form-select';
import PromptEvalTargetFormSelect from '../../components/eval-target-prompt-form-select';
import usePromptDetail from './use-prompt-detail';
import { EvalTargetPromptDetail } from './eval-target-prompt-detail';
import { EvalTargetDynamicParams } from './dynamic-params/eval-target-dynamic-params';

const EvaluateTargetMappingFieldLabel = (
  <div className="inline-flex flex-row items-center">
    {'字段映射'}
    <Tooltip
      theme="dark"
      content="评测集字段到评测对象字段的映射，用于评测对象准确获取输入。"
    >
      <IconCozInfoCircle className="ml-1 w-4 h-4 coz-fg-secondary" />
    </Tooltip>
  </div>
);

/**
 * 评测对象, prompt 选择, 版本选择, 详情, 字段映射
 * @param props
 * @returns
 */
const PluginEvalTargetForm = (props: PluginEvalTargetFormProps) => {
  const {
    formValues,
    createExperimentValues,
    onChange,
    setCreateExperimentValues,
  } = props;

  const targetType = formValues.evalTargetType;

  const promptId = formValues.evalTarget || '';

  const sourceTargetVersion = formValues.evalTargetVersion || '';

  // 渲染数据
  const evaluationSetSchemas =
    createExperimentValues?.evaluationSetVersionDetail?.evaluation_set_schema
      ?.field_schemas;

  const { promptDetail, loading } = usePromptDetail({
    promptId: promptId as string,
    version: sourceTargetVersion,
  });

  const variableDefs =
    promptDetail?.prompt_commit?.detail?.prompt_template?.variable_defs;

  const variableList = useMemo(
    () =>
      variableDefs?.map(v => ({
        name: v.key,
        ...DEFAULT_TEXT_STRING_SCHEMA,
        ...v,
      })) || [],
    [variableDefs],
  );

  const handleEvalTargetChange = () => {
    onChange('evalTargetVersion', undefined);
    onChange('target_runtime_param', undefined);
  };

  const handleEvalTargetVersionChange = () => {
    onChange('evalTargetMapping', undefined);
    onChange('target_runtime_param', undefined);
  };

  useEffect(() => {
    if (variableList?.length > 0) {
      const payload = {};
      const currentMapping = formValues?.evalTargetMapping || {};
      // 构造初始数据 { input: '', output: ''}
      variableList.forEach(v => {
        // 如果当前 Mapping 有对应的值, 则直接使用当前的值
        payload[v?.name || ''] = currentMapping?.[v?.name || ''] || '';
      });
      onChange('evalTargetMapping', payload);
    }
    // 变量列表变了, 代表着所选的prompt或版本发生了变化
  }, [variableList]);

  return (
    <>
      {/* 类型存在时才使用 */}
      {targetType === EvalTargetType.CozeLoopPrompt ? (
        <>
          {/* prompt 选择 */}
          <PromptEvalTargetFormSelect
            className="w-full"
            field="evalTarget"
            onChangeWithObject={false}
            onChange={handleEvalTargetChange}
            filter={true}
          />

          {/* prompt 版本选择 */}
          <PromptEvalTargetVersionFormSelect
            promptId={promptId}
            sourceTargetVersion={sourceTargetVersion}
            className="w-full"
            field="evalTargetVersion"
            onChangeWithObject={false}
            onChange={handleEvalTargetVersionChange}
          />
          {/* prompt 详情 */}
          <Form.Slot noLabel>
            <EvalTargetPromptDetail
              promptDetail={promptDetail}
              loading={loading}
            />
          </Form.Slot>
          <EvaluateTargetMappingField
            field="evalTargetMapping"
            prefixField="evalTargetMapping"
            label={EvaluateTargetMappingFieldLabel}
            evaluationSetSchemas={evaluationSetSchemas}
            rules={[
              {
                required: true,
                validator: (_, value) => {
                  // 需要配置变量, 并且配置过字段映射
                  // 没有值, 或者为空对象
                  if (variableList?.length > 0 && isEmpty(value)) {
                    return new Error('请配置字段映射');
                  }
                  return true;
                },
                message: '请配置字段映射',
              },
            ]}
            loading={loading}
            keySchemas={variableList}
            selectProps={{
              prefix: '评测集',
            }}
          />
          <EvalTargetDynamicParams
            initialValue={formValues.target_runtime_param}
            promptDetail={promptDetail}
            onChange={v => {
              setCreateExperimentValues?.(prev => ({
                ...prev,
                target_runtime_param: v,
              }));
            }}
          />
        </>
      ) : null}
    </>
  );
};

export default PluginEvalTargetForm;
