/* eslint-disable complexity */
/* eslint-disable @coze-arch/use-error-in-catch */
import { type JSONSchema7, type JSONSchema7TypeName } from 'json-schema';
import JSONBig from 'json-bigint';
import Decimal from 'decimal.js';
import { safeJsonParse } from '@cozeloop/toolkit';
import {
  type Content,
  type FieldSchema,
} from '@cozeloop/api-schema/evaluation';
import { type MultiModalSpec } from '@cozeloop/api-schema/data';

import { ajv } from '@/utils/jsonschema-convert';
import { getDataType } from '@/utils/field-convert';

import {
  ContentType,
  DataType,
  DEFAULT_FILE_COUNT,
  DEFAULT_FILE_SIZE,
  DEFAULT_PART_COUNT,
  DEFAULT_SUPPORTED_FORMATS,
} from './type';
const jsonBig = JSONBig({ storeAsString: true });

export { getDataType };
Decimal.set({ precision: 300 });
Decimal.set({ toExpNeg: -7, toExpPos: 21 });

export const getColumnType = (fieldSchema?: FieldSchema): DataType => {
  if (fieldSchema?.content_type === ContentType.Text) {
    return getDataType(fieldSchema);
  }
  if (fieldSchema?.content_type === ContentType.MultiPart) {
    return DataType.MultiPart;
  }
  return DataType.String;
};

export const saftJsonParse = (value?: string) => {
  try {
    return JSON.parse(value || '');
  } catch (error) {
    return '';
  }
};

export const saftJsonBigParse = (jsonStr?: string) => {
  try {
    const parsed = jsonBig.parse(jsonStr || '');
    return parsed;
  } catch (error) {
    return '';
  }
};

export const getSchemaConfig = (schema?: string) => {
  const config = saftJsonBigParse(schema);
  return {
    multipleOf: config?.multipleOf,
    maximum: config?.maximum,
    minimum: config?.minimum,
  };
};

export const validateAndFormat = ({
  val,
  minimum,
  maximum,
  multipleOf,
}: {
  val: string;
  minimum?: Decimal;
  maximum?: Decimal;
  multipleOf?: Decimal;
}): string => {
  try {
    //去除str中不符合数字规范的内容，科学技术法要单独保留
    const newStr = val.replace(/[^\d.eE+-]/g, '');
    let decimalValue = new Decimal(newStr);
    // 检查范围
    if (minimum) {
      const minValue = new Decimal(minimum);
      if (decimalValue.lt(minValue)) {
        decimalValue = minValue;
      }
    }
    if (maximum) {
      const maxValue = new Decimal(maximum);
      if (decimalValue.gt(maxValue)) {
        decimalValue = maxValue;
      }
    }
    if (!multipleOf) {
      return decimalValue.toString();
    }
    // 调整到最近的 multipleOf 的倍数
    const multipleOfDecimal = new Decimal(multipleOf);
    decimalValue = decimalValue
      .div(multipleOfDecimal)
      .round()
      .mul(multipleOfDecimal);

    // 确定小数位数
    const decimalPlaces = Math.max(
      0,
      multipleOfDecimal.decimalPlaces(),
      decimalValue.decimalPlaces(),
    );
    let formattedStr = decimalValue.toFixed(decimalPlaces);
    if (formattedStr.includes('.')) {
      formattedStr = formattedStr.replace(/\.?0+$/, '');
    }

    return formattedStr;
  } catch {
    return '';
  }
};

export const validarDatasetItem = (
  value: string,
  callback: (error?: string) => void,
  fieldSchema?: FieldSchema,
) => {
  const type = getColumnType(fieldSchema);
  if (type !== DataType.Float && type !== DataType.Integer) {
    return true;
  }
  if (!/^-?(?:0|[1-9]\d*)(?:\.\d+)?$/.test(value)) {
    callback('请输入数字');
    return false;
  }
  // 校验value 是否为数字；
  let decimalValue;
  try {
    decimalValue = new Decimal(value);
    const { minimum, maximum, multipleOf } = getSchemaConfig(
      fieldSchema?.text_schema,
    );
    const minValue = minimum ? new Decimal(minimum) : undefined;
    const maxValue = maximum ? new Decimal(maximum) : undefined;
    if (minValue && decimalValue.lt(minValue)) {
      callback(`请输入大于等于${minimum}的数字`);
      return false;
    }
    if (maxValue && decimalValue.gt(maxValue)) {
      callback(`请输入小于等于${maximum}的数字`);
      return false;
    }

    if (type === DataType.Integer && decimalValue.isInteger() === false) {
      callback('请输入整数');
      return false;
    }
    if (type === DataType.Float && multipleOf) {
      const multipleOfDecimal = new Decimal(multipleOf);
      const division = decimalValue.dividedBy(multipleOfDecimal);
      if (!division.isInteger()) {
        callback(`仅支持精确到小数点后${multipleOfDecimal.decimalPlaces()}位`);
        return false;
      }
    }
    return true;
  } catch (error) {
    callback('请输入数字');
    return false;
  }
};

export const validateTextFieldData = (
  value: string,
  callback: (error?: string) => void,
  fieldSchema?: FieldSchema,
) => {
  try {
    const schema = safeJsonParse(fieldSchema?.text_schema);
    console.info(1111, schema);
    const type = getDataType(fieldSchema);
    const isRequired = fieldSchema?.isRequired;
    switch (type) {
      case DataType.Integer:
      case DataType.Float:
      case DataType.Boolean: {
        if (isRequired && (value === undefined || value === '')) {
          callback('请输入内容');
          return false;
        }
        break;
      }
      case DataType.Object:
      case DataType.ArrayString:
      case DataType.ArrayInteger:
      case DataType.ArrayFloat:
      case DataType.ArrayBoolean:
      case DataType.ArrayObject: {
        if (value === undefined || value === '') {
          if (isRequired) {
            callback('请输入内容');
            return false;
          }
          return true;
        }
        const data = safeJsonParse(value);
        if (typeof data !== 'object') {
          callback('输入内容不是合法json格式');
          return false;
        }
        const validate = ajv.compile(schema);
        const valid = validate(data);
        getSchemaErrorInfo(validate.errors);
        if (
          !valid &&
          validate?.errors?.some(
            error => error.keyword !== 'additionalProperties',
          )
        ) {
          const errorInfo = getSchemaErrorInfo(validate?.errors);
          callback(errorInfo);
          return false;
        }
        return true;
      }
      default: {
        return true;
      }
    }
    return true;
  } catch (error) {
    callback('请输入对象');
    return false;
  }
};

export const validateMultiPartData = (
  value: Array<Content>,
  callback: (error?: string) => void,
  fieldSchema?: FieldSchema,
) => {
  const checkImageError = value?.some((item: Content) => {
    if (item.content_type === ContentType.Image && !item?.image?.uri) {
      return true;
    }
    return false;
  });
  if (checkImageError) {
    callback('');
    return false;
  }
  return true;
};

export const getSchemaErrorInfo = (errors: Object | null | undefined) => {
  if (!errors) {
    return '输入内容不符合列的字段定义';
  }
  const errorInfo = errors?.[0];
  const type = errorInfo?.keyword;
  const instancePath = errorInfo?.instancePath;
  switch (type) {
    case 'type': {
      return `${instancePath}数据类型不符合字段定义`;
    }
    case 'required': {
      return `缺少必填字段"${instancePath ? `${instancePath}/` : ''}${errorInfo?.params?.missingProperty}"`;
    }
    case 'additionalProperties': {
      return `存在冗余字段${errorInfo?.params?.additionalProperty}`;
    }
    default: {
      return '输入内容不符合列的字段定义';
    }
  }
};

export const getDefaultValueByTypeAndSchema = (
  type: JSONSchema7TypeName,
  schema: JSONSchema7,
  onlyRequiredProperty = true,
) => {
  if (type === 'string') {
    return '';
  }
  if (type === 'integer' || type === 'number') {
    return 0;
  }
  if (type === 'boolean') {
    return false;
  }
  if (type === 'object') {
    return generateDefaultObject(schema, onlyRequiredProperty);
  }
  if (type === 'array') {
    return generateDefaultArray(schema, onlyRequiredProperty);
  }
  return null;
};

export const generateDefaultObject = (
  schema: JSONSchema7,
  onlyRequiredProperty = true,
) => {
  const result = {};
  const properties = schema?.properties || {};
  const required = schema?.required || [];
  const keyList = onlyRequiredProperty
    ? required
    : Object.keys(properties || {});
  keyList.forEach(key => {
    if (!properties?.[key]) {
      return;
    }
    const propSchema = (properties?.[key] || {}) as JSONSchema7;

    const propType = propSchema?.type as JSONSchema7TypeName;
    result[key] = getDefaultValueByTypeAndSchema(
      propType,
      propSchema,
      onlyRequiredProperty,
    );
  });
  return result;
};

export const generateDefaultArray = (
  schema: JSONSchema7,
  onlyRequiredProperty = true,
) => {
  // 如果你需要空数组默认值，直接返回 [];
  // return [];
  // 如果你希望有一个默认元素，可以这样生成:
  if (schema.items) {
    const itemSchema = schema.items as JSONSchema7;
    const itemType = itemSchema?.type as JSONSchema7TypeName;
    return [
      getDefaultValueByTypeAndSchema(
        itemType,
        itemSchema,
        onlyRequiredProperty,
      ),
    ];
  }
  return [];
};

export const generateDefaultBySchema = (
  fieldSchema: FieldSchema,
  onlyRequiredProperty = true,
) => {
  try {
    const schema = JSON.parse(fieldSchema.text_schema || '{}');
    if (schema.type === 'object') {
      const obj = generateDefaultObject(schema, onlyRequiredProperty);
      return JSON.stringify(obj, null, 2);
    }
    if (schema.type === 'array') {
      const obj = generateDefaultArray(schema, onlyRequiredProperty);
      return JSON.stringify(obj, null, 2);
    }
    return getDefaultValueByTypeAndSchema(
      schema.type,
      schema,
      onlyRequiredProperty,
    );
  } catch (error) {
    return '';
  }
};

export const getMultipartConfig = (multipartConfig?: MultiModalSpec) => {
  const { max_file_count, max_part_count, max_file_size, supported_formats } =
    multipartConfig || {};
  const maxFileCount = max_file_count
    ? Number(max_file_count)
    : DEFAULT_FILE_COUNT;
  const maxPartCount = max_part_count
    ? Number(max_part_count)
    : DEFAULT_PART_COUNT;
  const maxFileSize = max_file_size ? Number(max_file_size) : DEFAULT_FILE_SIZE;
  const supportedFormats = (
    supported_formats?.map(format => `.${format}`) || DEFAULT_SUPPORTED_FORMATS
  ).join(',');
  return { maxFileCount, maxPartCount, maxFileSize, supportedFormats };
};
