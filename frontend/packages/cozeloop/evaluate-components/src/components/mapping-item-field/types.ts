import { type FieldSchema } from '@cozeloop/api-schema/evaluation';

export type SchemaSourceType = 'set' | 'target';

export type OptionSchema = FieldSchema & {
  schemaSourceType: SchemaSourceType;
};

export interface OptionGroup {
  schemaSourceType: SchemaSourceType;
  children: OptionSchema[];
}

export const schemaSourceTypeMap = {
  set: '评测集',
  target: '评测对象',
};
