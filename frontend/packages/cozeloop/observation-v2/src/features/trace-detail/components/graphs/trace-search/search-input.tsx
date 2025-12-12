import React, { useState } from 'react';

import {
  type FilterField,
  FieldType,
  QueryType,
} from '@cozeloop/api-schema/observation';
import { IconCozMagnifier } from '@coze-arch/coze-design/icons';
import { Input, Select } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';

export interface SearchInputProps {
  value?: FilterField;
  onChange?: (filterField: FilterField) => void;
  placeholder?: string;
  className?: string;
}

const fieldOptions = [
  { label: 'Input', value: 'input' },
  { label: 'Output', value: 'output' },
  { label: 'Span Name', value: 'span_name' },
];

export const SearchInput: React.FC<SearchInputProps> = ({
  value,
  onChange,
  placeholder,
  className,
}) => {
  const { t } = useLocale();
  const [selectedField, setSelectedField] = useState<string>(
    value?.field_name || 'input',
  );

  const handleSearchChange = (searchValue: string) => {
    const newFilterField: FilterField = {
      field_name: selectedField,
      field_type: FieldType.String,
      query_type: QueryType.Match,
      values: [searchValue],
      is_custom: false,
    };

    onChange?.(newFilterField);
  };

  const handleFieldChange = (fieldName: string) => {
    setSelectedField(fieldName);

    // 如果当前有搜索值，立即更新FilterField
    const currentSearchValue = value?.values?.[0] || '';
    if (currentSearchValue) {
      const newFilterField: FilterField = {
        field_name: fieldName,
        field_type: FieldType.String,
        query_type: QueryType.Match,
        values: [currentSearchValue],
        is_custom: false,
      };
      onChange?.(newFilterField);
    }
  };

  // 从FilterField中提取搜索值
  const searchValue = value?.values?.[0] || '';

  const suffix = (
    <Select
      value={selectedField}
      onChange={selectedValue => handleFieldChange(selectedValue as string)}
      className="!border-0 min-w-fit w-fit"
      renderSelectedItem={item => (
        <div className="!text-[12px] ">{item.label}</div>
      )}
    >
      {fieldOptions.map(option => (
        <Select.Option
          key={option.value}
          value={option.value}
          className="!text-[12px] "
        >
          {option.label}
        </Select.Option>
      ))}
    </Select>
  );

  return (
    <>
      <Input
        value={searchValue}
        size="small"
        prefix={<IconCozMagnifier className="!w-[16px] !h-[16px]" />}
        onChange={handleSearchChange}
        placeholder={
          placeholder ||
          `${t('search')} ${
            fieldOptions.find(option => option.value === selectedField)?.label
          }`
        }
        className="!border-0 !bg-transparent !text-[12px]"
        showClear
      />
      {suffix}
    </>
  );
};
