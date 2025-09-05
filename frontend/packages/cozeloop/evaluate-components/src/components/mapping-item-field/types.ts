import { I18n } from '@cozeloop/i18n-adapter';
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
  set: I18n.t('evaluation_set'),
  target: I18n.t('evaluate_case_create_eval_object'),
};
