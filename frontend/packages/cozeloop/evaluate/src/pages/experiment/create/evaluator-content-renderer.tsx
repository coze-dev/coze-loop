/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { useMemo } from 'react';

import { DEFAULT_TEXT_STRING_SCHEMA } from '@cozeloop/evaluate-components';
import { EvaluatorType } from '@cozeloop/api-schema/evaluation';

import { type EvaluatorPro } from '@/types/experiment/experiment-create';
import { ReadonlyMappingItem } from '@/components/mapping-item-field/readonly-mapping-item';

import { CodeEvaluatorContent } from './code-evaluator-content';
import { I18n } from '@cozeloop/i18n-adapter';

interface EvaluatorContentRendererProps {
  evaluatorPro: EvaluatorPro;
  evaluatorType?: EvaluatorType;
}

/**
 * 根据评估器类型进行条件渲染的组件
 * - LLM 类型：渲染字段映射（ReadonlyMappingItem）
 * - Code 类型：渲染代码内容（CodeEvaluatorContent）
 */
export function EvaluatorContentRenderer({
  evaluatorPro,
  evaluatorType,
}: EvaluatorContentRendererProps) {
  // 类型判断逻辑：根据是否存在 code_evaluator 判断是否为 Code 评估器
  const isCodeEvaluator = useMemo(
    () => evaluatorType === EvaluatorType.Code,
    [evaluatorType],
  );

  // Code 评估器渲染
  if (isCodeEvaluator) {
    return (
      <CodeEvaluatorContent
        versionDetail={evaluatorPro.evaluatorVersionDetail}
        loading={false}
      />
    );
  }

  // LLM 评估器渲染（原有的字段映射逻辑）
  const inputSchemas =
    evaluatorPro?.evaluatorVersionDetail?.evaluator_content?.input_schemas ??
    [];

  return (
    <>
      <div className="text-sm font-medium coz-fg-primary mb-2">
        {I18n.t('field_mapping')}
      </div>
      <div className="flex flex-col gap-3">
        {inputSchemas.map(schema => (
          <ReadonlyMappingItem
            key={schema?.key}
            keyTitle={I18n.t('evaluator')}
            keySchema={{
              name: schema?.key,
              ...DEFAULT_TEXT_STRING_SCHEMA,
              content_type: schema.support_content_types?.[0],
              text_schema: schema.json_schema,
            }}
            optionSchema={evaluatorPro.evaluatorMapping?.[schema?.key ?? '']}
          />
        ))}
      </div>
    </>
  );
}
