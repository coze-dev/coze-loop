import cs from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  FormInput,
  Tooltip,
  useFieldApi,
  withField,
} from '@coze-arch/coze-design';

import { columnNameRuleValidator } from '@/utils/source-name-rule';

import { DataTypeSelect } from '../field-type';
import {
  DataType,
  getDataTypeListWithArray,
  type FieldObjectSchema,
} from '../../dataset-item/type';
import { RequiredField } from './required-field';
import { AdditionalPropertyField } from './additional-property-field';

import styles from './index.module.less';
interface JSONSchemaPropertyRenderProps {
  fieldKeyPrefix: string;
  level: number;
  parentFieldKey: string;
  disabled: boolean;
  showAdditional: boolean;
}
const FormRequiredField = withField(RequiredField);
const FormAdditionalPropertyField = withField(AdditionalPropertyField);
const MAXLEVEL = 4;
const FormDataTypeSelect = withField(DataTypeSelect);
export const JSONSchemaPropertyRender = ({
  fieldKeyPrefix,
  level,
  parentFieldKey,
  disabled,
  showAdditional,
}: JSONSchemaPropertyRenderProps) => {
  const isMaxLevel = level >= MAXLEVEL;
  const parentField = useFieldApi(parentFieldKey);
  const parentFieldValue = parentField.getValue() as FieldObjectSchema;
  const jsonField = useFieldApi(fieldKeyPrefix);
  const jsonValue = jsonField.getValue() as FieldObjectSchema;
  const renderDisabledLabel = (label: string) => (
    <Tooltip content={I18n.t('drilled_to_minimum_level_tips')}>{label}</Tooltip>
  );
  const isObject =
    jsonValue?.type === DataType.ArrayObject ||
    jsonValue?.type === DataType.Object;
  return (
    <div className={styles.container}>
      <FormInput
        label={I18n.t('name')}
        fieldClassName="flex-1"
        noLabel
        disabled={disabled}
        field={`${fieldKeyPrefix}.propertyKey`}
        rules={[
          {
            required: true,
            message: I18n.t('please_enter_column_name'),
          },
          {
            validator: columnNameRuleValidator,
          },
          {
            validator: (_, value) => {
              if (!value) {
                return true;
              }
              const allChildrenData = parentFieldValue?.children;
              const hasSameName = !!allChildrenData
                ?.filter(data => data.key !== jsonValue?.key)
                ?.find(data => data.propertyKey === value);
              return !hasSameName;
            },
            message: I18n.t('column_name_exists'),
          },
        ]}
      ></FormInput>
      <FormDataTypeSelect
        noLabel
        disabled={disabled}
        fieldClassName="w-[160px]"
        treeData={getDataTypeListWithArray(isMaxLevel, renderDisabledLabel)}
        onChange={() => {
          jsonField.setValue({
            ...jsonValue,
            children: [],
          });
        }}
        field={`${fieldKeyPrefix}.type`}
        className="w-full"
        rules={[{ required: true, message: I18n.t('select_data_type') }]}
      ></FormDataTypeSelect>
      <FormRequiredField
        disabled={disabled}
        noLabel
        fieldClassName={cs('w-[60px]')}
        className="w-full"
        field={`${fieldKeyPrefix}.isRequired`}
      />

      <FormAdditionalPropertyField
        disabled={disabled || !isObject}
        noLabel
        hiddenValue={!isObject}
        fieldClassName={cs('w-[120px]', showAdditional ? '' : 'hidden')}
        className="w-full"
        field={`${fieldKeyPrefix}.additionalProperties`}
      />
    </div>
  );
};
