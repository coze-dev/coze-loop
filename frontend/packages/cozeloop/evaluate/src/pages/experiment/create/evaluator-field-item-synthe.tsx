// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import {
  type Evaluator,
  EvaluatorType,
  type EvaluatorVersion,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { type RuleItem } from '@coze-arch/coze-design';

import { EvaluatorFieldItemLLM } from './evaluator-field-item-llm';
import { CodeEvaluatorContent } from './code-evaluator-content';

interface EvaluatorFieldItemSyntheProps {
  arrayField: {
    field: string;
  };
  evaluator?: Evaluator;
  evaluatorType?: EvaluatorType;
  loading: boolean;
  versionDetail?: EvaluatorVersion;
  evaluationSetSchemas?: FieldSchema[];
  evaluateTargetSchemas?: FieldSchema[];
  getEvaluatorMappingFieldRules?: (k: FieldSchema) => RuleItem[];
}

export function EvaluatorFieldItemSynthe(props: EvaluatorFieldItemSyntheProps) {
  const {
    arrayField,
    evaluatorType,
    loading,
    evaluator,
    versionDetail,
    evaluationSetSchemas,
    evaluateTargetSchemas,
    getEvaluatorMappingFieldRules,
  } = props;

  // 根据versionDetail中的type字段判断渲染内容
  const isCodeEvaluator = useMemo(
    () => evaluatorType === EvaluatorType.Code,
    [evaluatorType],
  );

  // code 评估器
  if (isCodeEvaluator) {
    return (
      <CodeEvaluatorContent loading={loading} versionDetail={versionDetail} />
    );
  }

  // 默认渲染 LLM 评估器
  return (
    <EvaluatorFieldItemLLM
      arrayField={arrayField}
      loading={loading}
      evaluator={evaluator}
      versionDetail={versionDetail as EvaluatorVersion}
      evaluationSetSchemas={evaluationSetSchemas}
      evaluateTargetSchemas={evaluateTargetSchemas}
      getEvaluatorMappingFieldRules={getEvaluatorMappingFieldRules}
    />
  );
}
