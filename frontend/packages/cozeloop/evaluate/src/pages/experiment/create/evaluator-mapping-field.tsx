// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type FC, useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  getTypeText,
  getInputTypeText,
  type SchemaSourceType,
  getDataType,
} from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozEmpty } from '@coze-arch/coze-design/icons';
import {
  type CommonFieldProps,
  EmptyState,
  Loading,
  type RuleItem,
  withField,
} from '@coze-arch/coze-design';

import {
  type OptionSchema,
  type OptionGroup,
  type ExpandedProperty,
} from '@/components/mapping-item-field/types';
import { InputMappingItemField } from '@/components/mapping-item-field/input-mapping-item-field';

import emptyStyles from './empty-state.module.less';

export interface EvaluatorMappingProps {
  loading?: boolean;
  keySchemas?: FieldSchema[];
  evaluationSetSchemas?: FieldSchema[];
  evaluateTargetSchemas?: FieldSchema[];
  prefixField: string;
  value?: Record<string, OptionSchema>;
  onChange?: (v?: Record<string, OptionSchema>) => void;
  getEvaluatorMappingFieldRules?: (k: FieldSchema) => RuleItem[];
}

// 解析JSON Schema并拍平Object的properties
function parseTextSchema(
  textSchema: string,
  rootPrefix = '',
  sourceType = 'set' as SchemaSourceType,
): ExpandedProperty[] {
  try {
    const schema = JSON.parse(textSchema);

    if (schema.type !== 'object' || !schema.properties) {
      return [];
    }

    const expandedProperties: ExpandedProperty[] = [];

    function flattenProperties(
      properties: Record<string, unknown>,
      currentPrefix = '',
    ) {
      Object.entries(properties).forEach(([key, value]) => {
        const valueObj = value as Record<string, unknown>;
        const fullPath = currentPrefix ? `${currentPrefix}.${key}` : key;
        const nameWithRoot = rootPrefix
          ? `${rootPrefix}.${fullPath}`
          : fullPath;

        // 创建当前级别的条目
        expandedProperties.push({
          key: nameWithRoot,
          name: nameWithRoot,
          label: fullPath, // label 不包含 rootPrefix
          type:
            valueObj?.type === 'array'
              ? `array<${(valueObj?.items as unknown as { type: string })?.type}>`
              : (valueObj.type as string) || 'unknown',
          schemaSourceType: sourceType,
          description: valueObj.description as string,
        });

        // 如果是object类型且有properties，递归处理
        if (valueObj.type === 'object' && valueObj.properties) {
          flattenProperties(
            valueObj.properties as Record<string, unknown>,
            fullPath,
          );
        }
      });
    }

    flattenProperties(schema.properties);
    return expandedProperties;
  } catch (error) {
    console.error('Failed to parse text_schema:', error);
    return [];
  }
}

export const EvaluatorMappingField: FC<
  CommonFieldProps & EvaluatorMappingProps
> = withField(function (props: EvaluatorMappingProps) {
  const {
    loading,
    keySchemas,
    evaluationSetSchemas,
    evaluateTargetSchemas,
    prefixField,
    getEvaluatorMappingFieldRules,
  } = props;
  const optionGroups = useMemo(() => {
    const res: OptionGroup[] = [];
    if (evaluationSetSchemas) {
      res.push({
        schemaSourceType: 'set',
        children: evaluationSetSchemas.map(s => {
          const type = getDataType(s);
          const payload = {
            ...s,
            schemaSourceType: 'set' as SchemaSourceType,
            fieldType: type,
            expandedProperties: s.text_schema
              ? parseTextSchema(s.text_schema as string, s.name, 'set')
              : [],
          };
          return payload;
        }),
      });
    }
    if (evaluateTargetSchemas && evaluateTargetSchemas.length > 0) {
      res.push({
        schemaSourceType: 'target',
        children: evaluateTargetSchemas.map(s => {
          const type = getDataType(s);
          const payload = {
            ...s,
            schemaSourceType: 'target' as SchemaSourceType,
            fieldType: type,
            expandedProperties: s.text_schema
              ? parseTextSchema(s.text_schema as string, s.name, 'target')
              : [],
          };
          return payload;
        }),
      });
    }
    return res;
  }, [evaluationSetSchemas, evaluateTargetSchemas]);

  if (loading) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <Loading
          className="!w-full"
          size="large"
          label={I18n.t('loading_field_mapping')}
          loading={true}
        ></Loading>
      </div>
    );
  }

  if (!keySchemas) {
    return (
      <div className="h-[84px] w-full flex items-center justify-center">
        <EmptyState
          size="default"
          icon={<IconCozEmpty className="coz-fg-dim text-32px" />}
          title={I18n.t('no_data')}
          className={emptyStyles['empty-state']}
          // description={I18n.t('select_evaluator_and_version_number_to_view')}
        />
      </div>
    );
  }

  return (
    <div>
      {keySchemas?.map(k => (
        <InputMappingItemField
          key={k.name}
          noLabel
          field={`${prefixField}.${k.name}`}
          fieldClassName="!pt-0"
          keyTitle={I18n.t('evaluator')}
          keySchema={k}
          optionGroups={optionGroups}
          rules={
            getEvaluatorMappingFieldRules
              ? getEvaluatorMappingFieldRules(k)
              : [
                  {
                    // v 为 wf 字段, k 为 评测集列字段
                    validator: (_rule, v) => {
                      if (!v) {
                        return new Error(
                          I18n.t('please_select', { field: '' }),
                        );
                      }
                      if (
                        getTypeText(v) !== getTypeText(k) &&
                        getInputTypeText(v) !== getTypeText(k)
                      ) {
                        return new Error(
                          I18n.t('selected_fields_inconsistent'),
                        );
                      }
                      return true;
                    },
                  },
                ]
          }
        />
      ))}
    </div>
  );
});
