// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { cloneDeep } from 'lodash-es';
import { DEFAULT_TEXT_STRING_SCHEMA } from '@cozeloop/evaluate-components';
import {
  type EvalTarget,
  type EvalTargetVersion,
  type EvaluationSet,
  type EvaluationSetVersion,
  type Experiment,
  type EvaluatorFieldMapping,
  type SubmitExperimentRequest,
  type EvalTargetType,
} from '@cozeloop/api-schema/evaluation';
import { DatasetStatus } from '@cozeloop/api-schema/data';

import {
  type EvaluatorPro,
  type CreateExperimentValues,
  type EvaluatorValues,
} from '@/types/experiment/experiment-create';
import { type OptionSchema } from '@/components/mapping-item-field/types';

export function isEvalTargetDelete(target: EvalTarget | undefined) {
  if (!target?.source_target_id) {
    return true;
  }
  return Boolean(target?.base_info?.deleted_at);
}

function isEvalSetDelete(set: EvaluationSet | undefined) {
  if (!set?.id) {
    return true;
  }
  return Boolean(set?.status === DatasetStatus.Deleted);
}

export const getCurrentTime = () => new Date().getTime();

/**
 * 这个函数用于转换表单数据，主要完成以下工作：
 * 1. 提取所有评估器版本ID，存入evaluator_version_ids数组
 * 2. 处理评估器的label属性，确保其值为name
 * 3. 构建评估器字段映射(evaluator_field_mapping)，包括：
 *    - 从评测集(from_eval_set)映射的字段
 *    - 从评测对象(from_target)映射的字段
 * 4. 返回转换后的完整表单数据，用于API提交
 *
 * 注意：根据TODO注释，这个函数应该被移到表单监听中统一处理
 */
const getEvaluatorSubmitValues = (evaluatorProList: EvaluatorPro[]) => {
  const evaluatorVersionIds: EvaluatorValues['evaluator_version_ids'] = [];
  const evaluatorFieldMapping: EvaluatorValues['evaluator_field_mapping'] = [];

  evaluatorProList?.forEach(evaluatorPro => {
    const versionId = evaluatorPro?.evaluatorVersion?.id;
    // @ts-expect-error label是ReactNode类型，需要删除但会导致类型错误，这里忽略类型检查
    if (evaluatorPro?.evaluator?.label) {
      // @ts-expect-error 同上，处理label属性时忽略类型检查
      evaluatorPro.evaluator.label = evaluatorPro?.evaluator?.name;
    }

    if (versionId) {
      evaluatorVersionIds.push(versionId);

      const evaluatorFieldMappingItem: Required<EvaluatorFieldMapping> = {
        evaluator_version_id: versionId,
        from_eval_set: [],
        from_target: [],
      };

      Object.entries(evaluatorPro?.evaluatorMapping || {}).forEach(([k, v]) => {
        switch (v.schemaSourceType) {
          case 'set':
            evaluatorFieldMappingItem.from_eval_set.push({
              field_name: k,
              from_field_name: v.name,
            });
            break;
          case 'target':
            evaluatorFieldMappingItem.from_target.push({
              field_name: k,
              from_field_name: v.name,
            });
            break;
          default:
            break;
        }
      });

      evaluatorFieldMapping.push(evaluatorFieldMappingItem);
    }
  });

  return {
    evaluator_version_ids: evaluatorVersionIds,
    evaluator_field_mapping: evaluatorFieldMapping,
  };
};

export function experimentToCreateExperimentValues(params: {
  experiment: Experiment;
  spaceID: string;
}): CreateExperimentValues {
  const { experiment, spaceID } = params;
  const { eval_set = {}, eval_target = {} } = experiment;
  const { evaluation_set_version = {} } = eval_set;
  const { eval_target_version = {} } = eval_target;
  const evalSetDelete = isEvalSetDelete(eval_set);
  const targetDelete = isEvalTargetDelete(eval_target);

  // 保留label, 与渲染无关, 同时历史数据已经存了 label
  const datasetValues: Omit<CreateExperimentValues, 'workspace_id'> =
    evalSetDelete
      ? {}
      : {
          evaluationSet: eval_set.id,
          evaluationSetDetail: eval_set,
          evaluationSetVersion: evaluation_set_version.id,
          evaluationSetVersionDetail: evaluation_set_version,
        };
  const evalTargetValues: Omit<CreateExperimentValues, 'workspace_id'> =
    targetDelete
      ? {}
      : {
          evalTarget: eval_target?.source_target_id,
          evalTargetVersion: eval_target_version?.source_target_version,
          evalTargetMapping: experiment2EvalTargetMapping(experiment),
        };
  const values: CreateExperimentValues = {
    ...experiment,
    ...datasetValues,
    ...evalTargetValues,
    workspace_id: spaceID,
    // 评测对象 id
    evalTarget: eval_target?.source_target_id,
    // 评测对象类型
    evalTargetType: eval_target.eval_target_type,
    evaluatorProList: experiment2evaluatorProList(experiment),
    evalTargetVersionDetail: eval_target_version,
  };

  return values;
}

function experiment2EvalTargetMapping(
  experiment: Experiment,
): Record<string, OptionSchema> {
  const { target_field_mapping, eval_set } = experiment;
  const setFieldSchemas =
    eval_set?.evaluation_set_version?.evaluation_set_schema?.field_schemas;
  const result: Record<string, OptionSchema> = {};
  target_field_mapping?.from_eval_set?.forEach(item => {
    const fromFieldName = item.from_field_name ?? '';
    const fieldName = item.field_name ?? '';
    const setFieldSchema = setFieldSchemas?.find(s => s.name === fromFieldName);
    result[fieldName] = {
      ...setFieldSchema,
      name: fromFieldName,
      schemaSourceType: 'set',
    };
  });
  return result;
}

function experiment2evaluatorProList(experiment: Experiment) {
  const { evaluators, evaluator_field_mapping, eval_set } = experiment;
  const setFieldSchemas =
    eval_set?.evaluation_set_version?.evaluation_set_schema?.field_schemas;

  const clonedEvaluators = cloneDeep(evaluators);

  const result = clonedEvaluators?.map(evaluator => {
    // 如果evaluator被删除了，就不显示在evaluatorProList中
    const evaluatorDelete = Boolean(evaluator?.base_info?.deleted_at);
    if (evaluatorDelete) {
      return {};
    }
    const experimentMapping = evaluator_field_mapping?.find(
      m => m.evaluator_version_id === evaluator.current_version?.id,
    );
    const evaluatorMapping: EvaluatorPro['evaluatorMapping'] = {};

    experimentMapping?.from_eval_set?.forEach(item => {
      const fromFieldName = item.from_field_name ?? '';
      const fieldName = item.field_name ?? '';
      const setFieldSchema = setFieldSchemas?.find(
        s => s.name === fromFieldName,
      );
      evaluatorMapping[fieldName] = {
        ...setFieldSchema,
        name: fromFieldName,
        schemaSourceType: 'set',
      };
    });

    experimentMapping?.from_target?.forEach(item => {
      const fromFieldName = item.from_field_name ?? '';
      const fieldName = item.field_name ?? '';
      evaluatorMapping[fieldName] = {
        name: fromFieldName,
        schemaSourceType: 'target',
        ...DEFAULT_TEXT_STRING_SCHEMA,
      };
    });

    return {
      evaluator: {
        ...evaluator,
        label: evaluator.name,
        value: evaluator.evaluator_id,
      },
      evaluatorVersion: {
        ...(evaluator.current_version ?? {}),
        label: evaluator.current_version?.version,
        value: evaluator.current_version?.id,
      },
      evaluatorVersionDetail: evaluator.current_version,
      evaluatorMapping,
    };
  });

  return result;
}

export function evaluationSetToCreateExperimentValues(
  evaluationSet: EvaluationSet,
  evaluationSetVersion: EvaluationSetVersion,
  spaceID: string,
): CreateExperimentValues {
  const values: CreateExperimentValues = {
    workspace_id: spaceID,
    evaluatorProList: [{}],
    evaluationSet: evaluationSet.id,
    evaluationSetVersion: evaluationSetVersion.id,
    evaluationSetVersionDetail: evaluationSetVersion as EvaluationSetVersion,
    evaluationSetDetail: evaluationSet as EvaluationSet,
  };
  return values;
}

export const defaultGetTargetOption = (item: EvalTarget) => ({
  value: item?.source_target_id || '',
  label:
    item.eval_target_version?.eval_target_content?.prompt?.prompt_key || '',
});

export const defaultGetTargetVersionOption = (item: EvalTargetVersion) => ({
  value: item?.source_target_version || '',
  label: item.eval_target_content?.prompt?.prompt_key || '',
});

const getTargetFieldMapping = (values: CreateExperimentValues) => {
  const { evalTargetMapping = {} } = values;
  // 没有选择评测对象, 就是使用了评测集作为评测对象, 直接返回 undefined
  if (!Object.keys(evalTargetMapping).length) {
    return undefined;
  }

  return {
    from_eval_set: Object.entries(evalTargetMapping).map(([k, v]) => ({
      // 字段名称
      field_name: k,
      // 字段来源
      from_field_name: v?.name || v.key,
      // from_field_name: v.key,
    })),
  };
};

export const getSubmitValues = (
  values: CreateExperimentValues,
): SubmitExperimentRequest => {
  const newValues = {
    ...values,
    evaluatorProList: values.evaluatorProList?.map(ep => ({
      ...ep,
      evaluator: {
        ...ep.evaluator,
        label: ep?.evaluator?.name,
      },
      evaluatorVersion: {
        ...ep.evaluatorVersion,
        label: ep?.evaluatorVersion?.version,
      },
    })),
  };
  const clonedValues = cloneDeep(newValues);
  const createEvalTargetParam = getEvaluatorSubmitValues(
    clonedValues.evaluatorProList || [],
  );

  // 请求中会 pick 需要的数据, 没必要置为 undefined
  const result = {
    ...clonedValues,
    ...createEvalTargetParam,
    eval_set_id: values?.evaluationSet,
    eval_set_version_id: values?.evaluationSetVersion,
  };

  // 服务端参数对齐, 如果选择了评测对象, 则需要设置 create_eval_target_param, 否则不设置
  if (values?.evalTargetType) {
    result.create_eval_target_param = {
      eval_target_type: clonedValues?.evalTargetType as EvalTargetType,
      source_target_id: clonedValues?.evalTarget,
      source_target_version: values?.evalTargetVersion,
    };
  }

  const targetFieldMapping = getTargetFieldMapping(values);

  // 如果选择了评测对象, 则需要设置 target_field_mapping
  if (targetFieldMapping) {
    result.target_field_mapping = targetFieldMapping;
  }
  return result;
};

// 默认校验内容
const defaultValidFields = {
  0: ['name', 'desc', 'item_concur_num'],
  1: ['evaluationSet', 'evaluationSetVersion'],
  2: ['evalTargetType'],
  3: ['evaluatorProList'],
};

export const getValidateFields = ({
  currentStep,
  extraFields,
  values,
}: {
  currentStep: number;
  extraFields: (values: CreateExperimentValues) => string[];
  values: CreateExperimentValues;
}) => {
  if (typeof extraFields === 'function') {
    return extraFields(values);
  } else {
    const defaultArray = defaultValidFields?.[currentStep] || [];
    const extraArray = extraFields || [];
    return [...defaultArray, ...extraArray];
  }
};
