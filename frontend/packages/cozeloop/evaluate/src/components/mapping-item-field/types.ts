import { I18n } from '@cozeloop/i18n-adapter';
export type SchemaSourceType = 'set' | 'target';

export interface ExpandedProperty {
  key: string;
  name?: string;
  label: string;
  type: string;
  description?: string;
  schemaSourceType?: SchemaSourceType;
}

export interface OptionSchema {
  name?: string;
  description?: string;
  schemaSourceType: SchemaSourceType;
  expandedProperties?: ExpandedProperty[];
  fieldType?: string;
}

export interface OptionGroup {
  schemaSourceType: SchemaSourceType;
  children: OptionSchema[];
}

export const schemaSourceTypeMap = {
  set: I18n.t('evaluation_set'),
  target: I18n.t('evaluation_object_target'),
};
