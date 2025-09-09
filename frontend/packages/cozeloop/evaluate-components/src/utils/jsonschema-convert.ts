// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable eslint-comments/require-description */
import { nanoid } from 'nanoid';
import { type JSONSchema7TypeName, type JSONSchema7 } from 'json-schema';
import Ajv from 'ajv';
import { I18n } from '@cozeloop/i18n-adapter';

import {
  type DataType,
  type FieldObjectSchema,
} from '@/components/dataset-item/type';

import { getDataType } from './field-convert';
export const convertFieldObjectToSchema = (
  data: FieldObjectSchema,
): JSONSchema7 => {
  const { children, type } = data || {};
  const isArray = type?.includes('array');
  const arrayType = type
    ?.replace('array<', '')
    ?.replace('>', '') as unknown as JSONSchema7TypeName;
  const schemType: JSONSchema7TypeName = type?.includes('array')
    ? 'array'
    : (type as 'string' | 'integer' | 'boolean' | 'object' | 'number');
  const schema: JSONSchema7 = isArray
    ? {
        type: 'array',
        items: {
          type: arrayType,
          properties: {},
          required: [],
          ...(data?.additionalProperties !== undefined
            ? { additionalProperties: data?.additionalProperties }
            : {}),
        },
      }
    : {
        type: schemType,
        properties: {},
        ...(data?.additionalProperties !== undefined
          ? { additionalProperties: data?.additionalProperties }
          : {}),
      };
  if (children) {
    children.forEach(item => {
      if (isArray) {
        const items = schema.items as JSONSchema7;
        if (items.properties && item.propertyKey) {
          items.properties[item.propertyKey] = convertFieldObjectToSchema(item);
          if (item.isRequired) {
            items.required = [...(items.required || []), item.propertyKey];
          }
        }
      } else {
        if (schema.properties && item.propertyKey) {
          schema.properties[item.propertyKey] =
            convertFieldObjectToSchema(item);
          if (item.isRequired) {
            schema.required = [...(schema.required || []), item.propertyKey];
          }
        }
      }
    });
  }
  return schema;
};
export const convertJSONSchemaToFieldObject = (
  data: JSONSchema7,
  isRequired?: boolean,
): FieldObjectSchema => {
  const { type, properties } = data;
  const isArray = type === 'array';
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fieldType: any = isArray
    ? `array<${(data?.items as JSONSchema7)?.type}>`
    : type;
  const required = isArray
    ? (data?.items as JSONSchema7)?.required
    : data?.required;
  const additionalProperties = isArray
    ? (data?.items as JSONSchema7)?.additionalProperties
    : data?.additionalProperties;
  const fieldObject: FieldObjectSchema = {
    type: fieldType,
    key: nanoid(),
    propertyKey: '',
    children: [],
    additionalProperties: additionalProperties === false ? false : true,
    isRequired,
  };
  const schemaProperties = isArray
    ? (data?.items as JSONSchema7)?.properties
    : properties;
  if (schemaProperties) {
    fieldObject.children = Object.keys(schemaProperties).map(key => ({
      ...convertJSONSchemaToFieldObject(
        schemaProperties[key] as JSONSchema7,
        required?.includes(key),
      ),
      propertyKey: key,
    }));
  }
  return fieldObject;
};

export const ajv = new Ajv();

export const isValidSchema = (schema: Object) => {
  try {
    const res = ajv.validateSchema(schema);
    return res;
  } catch (error) {
    return false;
  }
};

export const validColumnSchema = ({
  schema,
  type,
  cb,
}: {
  schema: string;
  type?: DataType;
  cb?: (error: string) => void;
}) => {
  try {
    const schemaObj = JSON.parse(schema);
    const vaild = isValidSchema(schemaObj);
    if (!vaild) {
      cb?.(I18n.t('cozeloop_open_evaluate_json_schema_format_error'));
      return false;
    }
    if (type) {
      const schemaType = getDataType({
        text_schema: schema,
      });
      if (schemaType !== type) {
        cb?.(I18n.t('cozeloop_open_evaluate_json_schema_type_mismatch'));
        return false;
      }
    }
    return true;
  } catch (error) {
    cb?.(I18n.t('cozeloop_open_evaluate_json_format_error'));
    return false;
  }
};
