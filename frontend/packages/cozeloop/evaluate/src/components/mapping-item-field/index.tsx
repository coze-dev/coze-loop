import { type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  EqualItem,
  ReadonlyItem,
  getTypeText,
  TypographyText,
} from '@cozeloop/evaluate-components';
import { BaseSearchSelect } from '@cozeloop/components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Select,
  Tag,
  Tooltip,
  withField,
  type CommonFieldProps,
} from '@coze-arch/coze-design';

import {
  type OptionGroup,
  type OptionSchema,
  schemaSourceTypeMap,
} from './types';

import styles from './index.module.less';

const separator = '--';

function getGroupKey(group: OptionGroup) {
  const childrenNames = group.children?.map(e => e.name)?.join(',') ?? '';
  return group.schemaSourceType + childrenNames;
}

export interface MappingItemProps {
  keyTitle?: string;
  keySchema?: FieldSchema;
  optionGroups?: OptionGroup[];
  value?: OptionSchema;
  onChange?: (v?: OptionSchema) => void;
  validateStatus?: 'error';
}

export const MappingItemField: FC<CommonFieldProps & MappingItemProps> =
  withField(function ({
    keyTitle,
    keySchema,
    optionGroups,
    value,
    onChange,
    validateStatus,
  }: MappingItemProps) {
    const selectValue = value
      ? `${value.schemaSourceType}${separator}${value.name}`
      : undefined;
    const handleChange = (v?: string) => {
      const [schemaSourceType, name] = v?.split(separator) || [];
      const selectGroup = schemaSourceType
        ? optionGroups?.find(g => g.schemaSourceType === schemaSourceType)
        : undefined;
      const selectOptionSchema = name
        ? selectGroup?.children.find(s => s.name === name)
        : undefined;

      onChange?.(selectOptionSchema);
    };

    return (
      <div className="flex flex-row items-center gap-2">
        <ReadonlyItem
          className="flex-1"
          title={keyTitle}
          typeText={getTypeText(keySchema)}
          value={keySchema?.name}
        />
        <EqualItem />
        <BaseSearchSelect
          validateStatus={validateStatus}
          className={styles.select}
          placeholder={I18n.t('please_select', { field: '' })}
          prefix={
            value?.schemaSourceType &&
            schemaSourceTypeMap[value.schemaSourceType]
          }
          suffix={
            value?.fieldType && (
              <Tag size="mini" color="primary">
                {getTypeText(value)}
              </Tag>
            )
          }
          value={selectValue}
          onChange={v => handleChange(v as string)}
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
        </BaseSearchSelect>
      </div>
    );
  });
