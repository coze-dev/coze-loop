/* eslint-disable complexity */
import { cloneDeep } from 'lodash-es';
import { safeJsonParse } from '@cozeloop/toolkit';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';

import {
  ContentType,
  DataType,
  type FieldObjectSchema,
  type ConvertFieldSchema,
  contentTypeToDataType,
  dataTypeToContentType,
} from '../components/dataset-item/type';
import {
  convertFieldObjectToSchema,
  convertJSONSchemaToFieldObject,
} from './jsonschema-convert';
export const getDefaultFieldData = () => ({
  type: DataType.String,
  additionalProperties: false,
});

export const getDataType = (fieldSchema?: FieldSchema) => {
  try {
    if (fieldSchema?.content_type === ContentType.MultiPart) {
      return DataType.MultiPart;
    }
    const json = JSON.parse(fieldSchema?.text_schema || '{}');
    const type =
      json.type === 'array' ? `array<${json?.items?.type}>` : json.type;
    if (Object.values(DataType).includes(type)) {
      return type;
    }
    return DataType.String;
  } catch (error) {
    console.error(error);
    return DataType.String;
  }
};

export const convertSchemaToDataType = (
  schema: FieldSchema,
): ConvertFieldSchema => {
  if (schema.content_type && schema.content_type !== ContentType.Text) {
    const dataType = contentTypeToDataType[schema.content_type];
    return {
      ...schema,
      type: dataType,
    };
  }

  let children;
  let additionalProperties;
  const dataType = getDataType(schema);
  if (dataType === DataType.Object || dataType === DataType.ArrayObject) {
    const schemaJSON = safeJsonParse(schema.text_schema);
    const fieldObj = convertJSONSchemaToFieldObject(schemaJSON);
    children = fieldObj?.children || [];
    additionalProperties = fieldObj?.additionalProperties;
  }
  return {
    ...schema,
    type: dataType,
    children,
    additionalProperties,
  };
};

export const convertDataTypeToSchema = (
  data: ConvertFieldSchema,
): FieldSchema => {
  if (data?.type === DataType.MultiPart || data?.type === DataType.Image) {
    return {
      ...data,
      content_type: dataTypeToContentType[data.type],
    };
  }
  let textSchema = '';
  if (data?.type === DataType.Object || data?.type === DataType.ArrayObject) {
    if (data.schema) {
      textSchema = data.schema;
    } else {
      const schemaObj = convertFieldObjectToSchema({
        type: data?.type,
        children: data?.children,
        key: '',
        additionalProperties: data?.additionalProperties,
      });
      textSchema = JSON.stringify(schemaObj, null, 2);
    }
  } else {
    textSchema = TYPE_CONFIG[data.type as DataType];
  }
  return {
    ...data,
    text_schema: textSchema,
  };
};

export const getDefaultShowAdvanceConfig = (fieldValue: ConvertFieldSchema) => {
  if (
    fieldValue?.default_transformations ||
    fieldValue?.text_schema?.includes('additionalProperties": false')
  ) {
    return true;
  }
  return false;
};
export const getColumnHasRequiredAndAdditional = (
  fieldValue: ConvertFieldSchema,
) => {
  let hasAdditionalProperties = false;
  const checkRequiredAndAdditional = (
    field: FieldObjectSchema | ConvertFieldSchema,
  ) => {
    if (field.additionalProperties) {
      hasAdditionalProperties = true;
    }
    if (field?.children?.length) {
      field?.children?.forEach(item => checkRequiredAndAdditional(item));
    }
  };
  checkRequiredAndAdditional(fieldValue);
  return {
    hasAdditionalProperties,
  };
};

function isAllowedType(type) {
  const allowed = ['number', 'array', 'integer', 'object', 'boolean', 'string'];
  return allowed.includes(type);
}

export const validateJsonSchemaV7Strict = (schema, depth = 1) => {
  try {
    if (depth > 5) {
      return false;
    }
    if (typeof schema !== 'object' || schema === null) {
      return true;
    }

    // 校验 type
    if (
      !schema.type ||
      typeof schema.type !== 'string' ||
      !isAllowedType(schema.type)
    ) {
      return false;
    }

    // 禁止 type 为数组或存在多个类型
    // 标准要求 type 可以是数组, 但本需求禁止
    if (Array.isArray(schema.type)) {
      return false;
    }

    // array 类型
    if (schema.type === 'array') {
      // 必须有 items
      if (!schema?.items) {
        return false;
      }
      // 禁止 items 为数组（tuple），只能为单一schema
      if (Array.isArray(schema.items)) {
        return false;
      }
      // items 必须是对象
      if (typeof schema.items !== 'object' || schema.items === null) {
        return false;
      }
      // items 的 type 不能是 array
      if (!schema.items.type || !isAllowedType(schema.items.type)) {
        return false;
      }
      if (schema.items.type === 'array') {
        return false;
      }
      // items.type 不能是数组
      if (Array.isArray(schema.items.type)) {
        return false;
      }
      // items递归校验
      if (!validateJsonSchemaV7Strict(schema.items, depth)) {
        return false;
      }
    }

    // object 类型
    if (schema.type === 'object') {
      if (schema.properties === undefined) {
        return true;
      }
      if (typeof schema.properties !== 'object') {
        return false;
      }
      for (const key in schema.properties) {
        if (!validateJsonSchemaV7Strict(schema.properties[key], depth + 1)) {
          return false;
        }
      }
    }
    // 其他类型不递归
    return true;
  } catch (error) {
    return false;
  }
};

export const resetAdditonalProperty = (fieldSchema: ConvertFieldSchema) => {
  const cloneFieldSchema = cloneDeep(fieldSchema);
  const trans = (schema: ConvertFieldSchema | FieldObjectSchema) => {
    if (!schema) {
      return;
    }
    schema.additionalProperties = false;
    if (schema?.children?.length) {
      schema.children.forEach(item => trans(item));
    }
  };
  trans(cloneFieldSchema);

  return cloneFieldSchema;
};
export const TYPE_CONFIG: Record<DataType, string> = {
  [DataType.Float]: '{"type":"number"}',
  [DataType.Integer]: '{"type":"integer"}',
  [DataType.String]: '{"type":"string"}',
  [DataType.Boolean]: '{"type":"boolean"}',
  [DataType.ArrayBoolean]: JSON.stringify({
    type: 'array',
    items: {
      type: 'boolean',
    },
  }),
  [DataType.ArrayString]: JSON.stringify({
    type: 'array',
    items: {
      type: 'string',
    },
  }),
  [DataType.ArrayFloat]: JSON.stringify({
    type: 'array',
    items: {
      type: 'number',
    },
  }),
  [DataType.ArrayInteger]: JSON.stringify({
    type: 'array',
    items: {
      type: 'integer',
    },
  }),
  [DataType.ArrayObject]: JSON.stringify({
    type: 'array',
    items: {
      type: 'object',
    },
  }),
  [DataType.Object]: JSON.stringify({
    type: 'object',
  }),
  [DataType.MultiPart]: '',
  [DataType.Image]: '',
};
