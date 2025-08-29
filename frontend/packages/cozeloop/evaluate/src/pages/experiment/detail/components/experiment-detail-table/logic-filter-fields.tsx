import { safeJsonParse } from '@cozeloop/toolkit';
import {
  getLogicFieldName,
  type LogicField,
} from '@cozeloop/evaluate-components';
import {
  type ColumnEvaluator,
  FieldType,
  type FieldSchema,
  ContentType,
  type ColumnAnnotation,
} from '@cozeloop/api-schema/evaluation';
import { tag } from '@cozeloop/api-schema/data';

import { EvaluatorColumnPreview } from '@/components/experiment';

import {
  IDSearchInput,
  InputLimitLengthHOC,
  TextAreaLimitLengthHOC,
} from './logic-filter-setter';

// 搜索最大长度
export const MAX_SEARCH_LENGTH = 10000;

function getEvalSetLogicField(fieldSchema: FieldSchema): LogicField {
  const { name = '', text_schema, key = '' } = fieldSchema ?? {};
  const jsonSchema = text_schema
    ? (safeJsonParse(text_schema) as { type: string } | undefined)
    : undefined;
  const schemaType = jsonSchema?.type;
  const setterProps: Record<string, unknown> = {};
  const logicField: LogicField = {
    title: name,
    name: getLogicFieldName(FieldType.EvalSetColumn, key),
    type: 'string',
    setterProps,
  };
  if (schemaType === 'integer' || schemaType === 'number') {
    logicField.type = 'number';
  } else if (schemaType === 'boolean') {
    logicField.type = 'options';
    logicField.customOperations = [
      { label: '等于', value: 'equals' },
      { label: '不等于', value: 'not-equals' },
    ];
    setterProps.multiple = false;
    setterProps.optionList = [
      { label: 'true', value: 'true' },
      { label: 'false', value: 'false' },
    ];
    logicField.disabledOperations = [];
  } else if (schemaType === 'object' || schemaType?.includes('array')) {
    // JSON类型数据使用输入框，不支持输入换行
    logicField.setter = InputLimitLengthHOC(MAX_SEARCH_LENGTH);
  } else if (schemaType === 'string') {
    logicField.setter = TextAreaLimitLengthHOC(MAX_SEARCH_LENGTH);
  }
  return logicField;
}

function getEvaluatorLogicField(evaluator: ColumnEvaluator): LogicField {
  const { evaluator_version_id: versionId = '' } = evaluator ?? {};
  const field: LogicField = {
    title: (
      <EvaluatorColumnPreview
        evaluator={evaluator}
        className="max-w-[200px] evaluator-preview-in-cascader"
      />
    ),
    name: getLogicFieldName(FieldType.EvaluatorScore, versionId),
    type: 'number',
    setterProps: { step: 0.1 },
  };
  return field;
}

function getAnnotationLogicField(
  columnAnnotation: ColumnAnnotation,
): LogicField {
  const {
    tag_key_name = '',
    tag_key_id = '',
    content_type,
    tag_values,
  } = columnAnnotation ?? {};
  const setterProps: Record<string, unknown> = {};
  const logicField: LogicField = {
    title: tag_key_name,
    name: getLogicFieldName(FieldType.AnnotationText, tag_key_id),
    type: 'string',
    setterProps,
  };
  if (content_type === tag.TagContentType.ContinuousNumber) {
    logicField.type = 'number';
    logicField.name = getLogicFieldName(FieldType.AnnotationScore, tag_key_id);
  } else if (content_type === tag.TagContentType.Categorical) {
    logicField.type = 'options';
    logicField.name = getLogicFieldName(
      FieldType.AnnotationCategorical,
      tag_key_id,
    );
    setterProps.multiple = true;
    setterProps.maxTagCount = 1;
    setterProps.optionList = tag_values?.map(item => ({
      label: item.tag_value_name,
      value: item.tag_value_id ?? item.id,
    }));
  } else if (content_type === tag.TagContentType.Boolean) {
    logicField.type = 'options';
    logicField.name = getLogicFieldName(
      FieldType.AnnotationCategorical,
      tag_key_id,
    );
    logicField.customOperations = [
      { label: '等于', value: 'equals' },
      { label: '不等于', value: 'not-equals' },
    ];
    setterProps.multiple = false;
    setterProps.optionList = tag_values?.map(item => ({
      label: item.tag_value_name,
      value: item.tag_value_id ?? item.id,
    }));
    logicField.disabledOperations = [];
  }
  return logicField;
}

export function getFilterFields(
  columnEvaluators: ColumnEvaluator[],
  fieldSchemas: FieldSchema[],
  columnAnnotations: ColumnAnnotation[],
) {
  const fields: LogicField[] = [
    {
      title: '评估器',
      name: 'evaluator',
      type: 'options',
      children: columnEvaluators.map(getEvaluatorLogicField),
    },
    {
      title: '评测集',
      name: 'eval_set',
      type: 'options',
      children: fieldSchemas
        ?.filter(item => item?.content_type === ContentType.Text)
        ?.map(getEvalSetLogicField),
    },
    {
      title: '人工标注',
      name: 'annotation',
      type: 'options',
      children: columnAnnotations?.map(getAnnotationLogicField),
    },
    {
      title: 'actual_output',
      name: getLogicFieldName(FieldType.ActualOutput, 'actual_output'),
      type: 'string',
      setter: TextAreaLimitLengthHOC(MAX_SEARCH_LENGTH),
    },
    {
      title: '是否人工校准得分',
      name: getLogicFieldName(FieldType.EvaluatorScoreCorrected, ''),
      type: 'options',
      customOperations: [
        { label: '等于', value: 'equals' },
        { label: '不等于', value: 'not-equals' },
      ],
      setterProps: {
        optionList: [
          { label: '是', value: '1' },
          { label: '否', value: '0' },
        ],
      },
    },
    {
      title: '数据项ID',
      name: getLogicFieldName(FieldType.ItemID, ''),
      type: 'string',
      setter: IDSearchInput,
      customOperations: [
        { label: '等于', value: 'equals' },
        { label: '不等于', value: 'not-equals' },
      ],
    },
  ];
  return fields;
}
