import { type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { Tag, withField, type CommonFieldProps } from '@coze-arch/coze-design';

import {
  type OptionGroup,
  type OptionSchema,
  schemaSourceTypeMap,
} from '../types';
import { EqualItem, ReadonlyItem, getTypeText } from '../../column-item-map';
import GroupSelect from './group-select';

import styles from './index.module.less';

const separator = '--';

export interface MappingItemProps {
  keyTitle?: string;
  keySchema?: FieldSchema;
  optionGroups?: OptionGroup[];
  value?: OptionSchema;
  onChange?: (v?: OptionSchema) => void;
  validateStatus?: 'error';
  disabled?: boolean;
}

export const InputSelectMappingItemField: FC<
  CommonFieldProps & MappingItemProps
> = withField(function (props: MappingItemProps) {
  const {
    keyTitle,
    keySchema,
    optionGroups,
    value,
    onChange,
    validateStatus,
    disabled = false,
  } = props;
  const selectValue = value
    ? `${value.schemaSourceType}${separator}${value.name}`
    : undefined;
  const handleChange = (v?: string) => {
    console.log('xxx vvvvvvv', v);
    const [schemaSourceType, name] = v?.split(separator) || [];
    const selectGroup = schemaSourceType
      ? optionGroups?.find(g => g.schemaSourceType === schemaSourceType)
      : undefined;
    const selectOptionSchema = name
      ? selectGroup?.children.find(s => s.name === name)
      : undefined;
    console.log('xxx selectOptionSchema', selectOptionSchema);
    onChange?.(selectOptionSchema);
  };

  console.log('xxx selectValue', selectValue);

  return (
    <div className="flex flex-row items-center gap-2">
      <ReadonlyItem
        className="flex-1"
        title={keyTitle}
        typeText={getTypeText(keySchema)}
        value={keySchema?.name}
      />
      <EqualItem />
      <GroupSelect
        validateStatus={validateStatus}
        className={styles.select}
        placeholder={I18n.t('please_select', { field: '' })}
        optionGroups={optionGroups}
        value={selectValue}
        disabled={disabled}
        onChange={handleChange}
        prefix={
          value?.schemaSourceType && schemaSourceTypeMap[value.schemaSourceType]
        }
        suffix={
          value?.content_type && (
            <Tag size="mini" color="primary">
              {getTypeText(value)}
            </Tag>
          )
        }
        renderSelectedItem={optionNode => {
          const [_, name] = optionNode?.value?.split(separator) || [];
          return name;
        }}
      />
      {/* <BaseSearchSelect
        validateStatus={validateStatus}
        className={styles.select}
        placeholder={I18n.t('please_select', { field: '' })}
        prefix={
          value?.schemaSourceType && schemaSourceTypeMap[value.schemaSourceType]
        }
        suffix={
          value?.content_type && (
            <Tag size="mini" color="primary">
              {getTypeText(value)}
            </Tag>
          )
        }
        value={selectValue}
        disabled={disabled}
        // @ts-expect-error semi类型问题
        onChange={handleChange}
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        renderSelectedItem={(optionNode: any) => {
          const [_, name] = optionNode?.value?.split(separator) || [];
          return name;
        }}
      >
        {optionGroups?.map(group => (
          <Select.OptGroup
            className="!border-0"
            label={
              <div className="ml-[-20px]">
                {schemaSourceTypeMap[group.schemaSourceType]}
              </div>
            }
            key={getGroupKey(group)}
          >
            {group.children.map(option => (
              <Select.Option
                value={`${option.schemaSourceType}${separator}${option.name}`}
                key={`${option.schemaSourceType}${separator}${option.name}`}
              >
                <div className="w-full flex flex-row items-center pl-2 gap-1 max-w-[330px]">
                  <TypographyText>{option.name}</TypographyText>
                  {option.description ? (
                    <Tooltip theme="dark" content={option.description}>
                      <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)] shrink-0" />
                    </Tooltip>
                  ) : null}
                  <Tag
                    className="mx-3 ml-auto shrink-0"
                    size="mini"
                    color="primary"
                  >
                    {getTypeText(option)}
                  </Tag>
                </div>
              </Select.Option>
            ))}
          </Select.OptGroup>
        ))}
      </BaseSearchSelect> */}
    </div>
  );
});
