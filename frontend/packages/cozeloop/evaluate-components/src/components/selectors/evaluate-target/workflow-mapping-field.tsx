/* eslint-disable complexity */

/* eslint-disable @typescript-eslint/no-explicit-any */
import { type RefObject, type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { ContentType, type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { withField, type CommonFieldProps } from '@coze-arch/coze-design';

import { type IKeySchema, type OptionSchema } from '@/types/evaluate-target';

import { type NodeData } from '../../../components/tree-editor/types';
import { TreeEditor } from '../../../components/tree-editor';
import { type OptionGroup } from '../../../components/mapping-item-field/types';
import { MappingItemField } from '../../../components/mapping-item-field';
import { getTypeText } from '../../../components/column-item-map';

export interface EvaluateTargetMappingProps {
  loading?: boolean;
  keySchemas?: IKeySchema[];
  prefixField: string;
  evaluationSetSchemas?: FieldSchema[];
  evaluationFieldReflectRef?: RefObject<Record<string, any>>;
  workflowInputsFieldReflectRef?: RefObject<Record<string, any>>;
  onFieldChange?: (field: any, value: any) => void;
}

/**
 * 将单个Object类型的schema转换为TreeEditor组件需要的NodeData格式
 * @param schema - 单个schema对象
 * @returns NodeData - 树形结构的节点
 */
function transformSingleSchemaToTreeData(schema: unknown): NodeData {
  /**
   * 将type转换为符合tree-test.tsx格式的类型
   */
  const convertType = (type?: string): string => {
    if (!type) {
      return 'String';
    }
    switch (type.toLowerCase()) {
      case 'string':
        return 'String';
      case 'object':
        return 'Object';
      case 'number':
      case 'integer':
        return 'Integer';
      case 'boolean':
        return 'Boolean';
      case 'float':
        return 'Float';
      default:
        return 'String';
    }
  };

  /**
   * 获取预览类型
   */
  const getPreviewType = (type?: string): string => {
    if (!type) {
      return 'PlainText';
    }
    switch (type.toLowerCase()) {
      case 'object':
        return 'JSON';
      default:
        return 'PlainText';
    }
  };

  /**
   * 递归处理嵌套的schema数据
   * @param schemaItem - 单个schema项，支持不同的数据格式
   * @param parentKey - 父节点的key，用于生成唯一的节点key
   * @returns NodeData - 转换后的节点数据
   */
  const processSchemaItem = (
    schemaItem: Record<string, unknown>,
    parentKey = '',
  ): NodeData => {
    // 获取字段名称，支持不同的字段名格式
    const fieldName =
      (schemaItem.name as string) || (schemaItem.key as string) || 'unnamed';

    // 生成唯一的节点key
    const nodeKey = parentKey ? `${parentKey}.${fieldName}` : fieldName;

    // 获取类型信息
    const fieldType = (schemaItem.type as string) || 'string';
    const convertedType = convertType(fieldType);
    const previewType = getPreviewType(fieldType);

    // 创建节点数据，格式与tree-test.tsx保持一致
    const nodeData: NodeData = {
      key: nodeKey,
      label: fieldName,
      data: {
        name: fieldName,
        type: convertedType,
        preview_type: previewType,
        required: (schemaItem.required as boolean) ?? false,
        description: schemaItem.description as string,
      },
    };

    // 处理子schema（支持不同的嵌套结构）
    const childSchemas =
      (schemaItem.schema as Record<string, unknown>[]) ||
      (schemaItem.children as Record<string, unknown>[]);
    if (
      childSchemas &&
      Array.isArray(childSchemas) &&
      childSchemas.length > 0
    ) {
      nodeData.children = childSchemas.map(
        (childSchema: Record<string, unknown>) =>
          processSchemaItem(childSchema, nodeKey),
      );
    }

    return nodeData;
  };

  // 直接处理单个schema，不需要创建额外的根节点
  return processSchemaItem(schema as Record<string, unknown>, '');
}

const getWorkflowMappingRules = (schema: FieldSchema) => [
  {
    validator: (_rule, v) => {
      if (!v) {
        return new Error(I18n.t('please_select'));
      }
      if (getTypeText(v) !== getTypeText(schema)) {
        return new Error(
          I18n.t('cozeloop_open_evaluate_selected_field_type_inconsistent'),
        );
      }
      return true;
    },
  },
];

const WorkflowMappingField: FC<CommonFieldProps & EvaluateTargetMappingProps> =
  withField((props: EvaluateTargetMappingProps) => {
    const {
      keySchemas,
      prefixField,
      evaluationSetSchemas,
      // onFieldChange,
      evaluationFieldReflectRef,
      workflowInputsFieldReflectRef,
    } = props;

    // 构建选项组数据，用于MappingItemField
    const optionGroups: OptionGroup[] = evaluationSetSchemas
      ? [
          {
            schemaSourceType: 'set',
            children: evaluationSetSchemas.map(schema => ({
              ...schema,
              schemaSourceType: 'set' as const,
            })),
          },
        ]
      : [];

    console.log('xxx keySchemas keySchemas', keySchemas);

    // 入参为 v 评测集列, field 评测对象字段
    const handleAfterChange = (v?: OptionSchema, field?: string) => {
      console.log('xxx handleAfterChange', v, field);
      if (
        !workflowInputsFieldReflectRef?.current ||
        !evaluationFieldReflectRef?.current ||
        !v ||
        !field
      ) {
        return;
      }
      console.log('xxx 11111111111111111');
      const setReflect =
        evaluationFieldReflectRef?.current?.[v?.name as string];
      // evalTargetMapping.xxxxxxx
      const wfField = field.split('.')[1];
      const wfReflect = workflowInputsFieldReflectRef?.current?.[wfField];

      if (
        !setReflect ||
        !wfReflect ||
        wfReflect?.type !== 'object' ||
        setReflect?.type !== 'object'
      ) {
        return;
      }

      const setMapping = setReflect?.mapping;
      const wfMapping = wfReflect?.mapping;

      const payloadArr: { targetField: string; value: any }[] = [];

      Object.entries(setMapping).forEach(([fieldPath, type]) => {
        // 字段相同, 且类型相同
        if (wfMapping[fieldPath] && type === wfMapping[fieldPath]) {
          payloadArr.push({
            targetField: `evalTargetMapping.${fieldPath}`,
            value: {
              key: fieldPath,
              name: fieldPath,
              status: 1,
              content_type: 'Text',
              description: I18n.t('evaluation_set_input_tips'),
              text_schema: JSON.stringify({ type: wfMapping[fieldPath] }),
              default_display_format: 5,
              schemaSourceType: 'set',
            },
          });
        }
      });

      console.log('xxx payloadArr', payloadArr);
    };

    return (
      <div>
        {keySchemas?.map((schema, index) => {
          // 判断是否为Object类型（包含嵌套结构）
          const isParameter = schema.name === 'parameter';

          if (isParameter) {
            // Object类型：使用TreeEditor渲染
            const treeData = transformSingleSchemaToTreeData(schema);

            return (
              <div key={(schema as FieldSchema).name || index}>
                <TreeEditor
                  treeData={treeData}
                  isShowAddNode={() => false}
                  isShowAction={false}
                  labelRender={({ nodeData, path }) => {
                    // 从nodeData中构建FieldSchema
                    const keySchema: FieldSchema = {
                      key: nodeData.key,
                      name: String(nodeData.data?.name || nodeData.label),
                      description: String(nodeData.data?.description || ''),
                      content_type: ContentType.Text,
                      text_schema: `{"type": "${String(nodeData.data?.type || 'string').toLowerCase()}"}`,
                      status: 1,
                      hidden: false,
                    };
                    const isChild = nodeData.key.includes('.');
                    // console.log('xxx 树结构的节点', nodeData.key, isChild);
                    return (
                      <MappingItemField
                        noLabel
                        disabled={isChild}
                        field={`${prefixField}.${nodeData.key}`}
                        fieldClassName="!pt-0"
                        keyTitle={I18n.t('evaluation_object')}
                        keySchema={keySchema}
                        optionGroups={optionGroups}
                        rules={getWorkflowMappingRules(keySchema)}
                        onAfterChange={handleAfterChange}
                      />
                    );
                  }}
                />
              </div>
            );
          } else {
            // 非Object类型：直接使用MappingItemField渲染
            return (
              <MappingItemField
                key={(schema as FieldSchema).name || index}
                noLabel
                field={`${prefixField}.${(schema as FieldSchema).name}`}
                fieldClassName="!pt-0"
                keyTitle={I18n.t('evaluation_object')}
                keySchema={schema as FieldSchema}
                optionGroups={optionGroups}
                rules={getWorkflowMappingRules(schema as FieldSchema)}
              />
            );
          }
        })}
      </div>
    );
  });

export default WorkflowMappingField;
