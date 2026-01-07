// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { I18n } from '@cozeloop/i18n-adapter';
import { EvaluatorVersionDetail } from '@cozeloop/evaluate-components';
import {
  type EvaluatorVersion,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import { type RuleItem, Tooltip } from '@coze-arch/coze-design';

import { EvaluatorMappingField } from './evaluator-mapping-field';

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
