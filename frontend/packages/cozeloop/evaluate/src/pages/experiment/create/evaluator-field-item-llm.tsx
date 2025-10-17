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

import { EvaluatorVersionDetail } from '@cozeloop/evaluate-components';
import {
  type EvaluatorVersion,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { type RuleItem, Tooltip } from '@coze-arch/coze-design';

import { EvaluatorMappingField } from './evaluator-mapping-field';
import { I18n } from '@cozeloop/i18n-adapter';

interface EvaluatorFieldItemLLMProps {
  arrayField: {
    field: string;
  };
  loading: boolean;
  versionDetail: EvaluatorVersion;
  evaluationSetSchemas?: FieldSchema[];
  evaluateTargetSchemas?: FieldSchema[];
  getEvaluatorMappingFieldRules?: (k: FieldSchema) => RuleItem[];
}

export function EvaluatorFieldItemLLM(props: EvaluatorFieldItemLLMProps) {
  const {
    arrayField,
    loading,
    versionDetail,
    evaluationSetSchemas,
    evaluateTargetSchemas,
    getEvaluatorMappingFieldRules,
  } = props;

  const keySchemas = versionDetail?.evaluator_content?.input_schemas?.map(
    item => ({
      name: item.key,
      content_type: item.support_content_types?.[0],
      text_schema: item.json_schema,
    }),
  );

  return (
    <>
      <EvaluatorVersionDetail loading={loading} versionDetail={versionDetail} />
      <EvaluatorMappingField
        field={`${arrayField.field}.evaluatorMapping`}
        prefixField={`${arrayField.field}.evaluatorMapping`}
        label={
          <div className="inline-flex flex-row items-center">
            {I18n.t('field_mapping')}
            <Tooltip
              theme="dark"
              content={I18n.t('evaluation_set_field_mapping_tip')}
            >
              <IconCozInfoCircle className="ml-1 w-4 h-4 coz-fg-secondary" />
            </Tooltip>
          </div>
        }
        loading={loading}
        keySchemas={keySchemas}
        evaluationSetSchemas={evaluationSetSchemas}
        evaluateTargetSchemas={evaluateTargetSchemas}
        getEvaluatorMappingFieldRules={getEvaluatorMappingFieldRules}
        rules={[
          {
            required: true,
            validator: (_, value) => {
              if (loading && !value) {
                return new Error(
                  I18n.t('evaluate_please_configure_field_mapping'),
                );
              }
              return true;
            },
          },
        ]}
      />
    </>
  );
}
