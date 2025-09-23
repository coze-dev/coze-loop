// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type FC } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  EqualItem,
  ReadonlyItem,
  getTypeText,
  getInputTypeText,
  type GetInputTypeTextParams,
} from '@cozeloop/evaluate-components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { Tag, withField, type CommonFieldProps } from '@coze-arch/coze-design';

import {
  type ExpandedProperty,
  type OptionGroup,
  type OptionSchema,
  schemaSourceTypeMap,
} from './types';
import GroupSelect from './group-select';

import styles from './index.module.less';

const separator = '--';

export interface MappingItemProps {
  keyTitle?: string;
  keySchema?: FieldSchema;
  optionGroups?: OptionGroup[];
  value?: OptionSchema;
  onChange?: (
    v?: OptionSchema | { name: string; schemaSourceType: string },
  ) => void;
  validateStatus?: 'error';
}

export const InputMappingItemField: FC<CommonFieldProps & MappingItemProps> =
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

    const handleChange = (
      v?: string,
      item?: OptionSchema | ExpandedProperty,
    ) => {
      const [schemaSourceType, name] = v?.split(separator) || [];
      onChange?.({ ...item, name, schemaSourceType });
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
        <GroupSelect
          validateStatus={validateStatus}
          className={styles.select}
          placeholder={I18n.t('please_select', { field: '' })}
          optionGroups={optionGroups}
          value={selectValue}
          onChange={handleChange}
          prefix={
            value?.schemaSourceType &&
            schemaSourceTypeMap[value.schemaSourceType]
          }
          suffix={
            value && (
              <Tag size="mini" color="primary" className="font-semibold">
                {getInputTypeText(value as unknown as GetInputTypeTextParams)}
              </Tag>
            )
          }
          renderSelectedItem={optionNode => {
            const [_, name] = optionNode?.value?.split(separator) || [];
            return name;
          }}
        />
      </div>
    );
  });
