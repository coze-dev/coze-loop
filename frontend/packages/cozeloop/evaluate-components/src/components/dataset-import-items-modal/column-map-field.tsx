// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { type FieldSchema } from '@cozeloop/api-schema/evaluation';
import { type FieldMapping } from '@cozeloop/api-schema/data';
import { Select, Typography } from '@coze-arch/coze-design';

import { EqualItem, getTypeText, ReadonlyItem } from '../column-item-map';

export interface FieldMappingConvert extends FieldMapping {
  description?: string;
  fieldSchema?: FieldSchema;
}
interface ColumnMapFieldProps {
  sourceColumns: string[];
  value?: FieldMappingConvert;
  onChange?: (value: FieldMappingConvert) => void;
}

export const ColumnMapField = ({
  sourceColumns,
  onChange,
  value,
}: ColumnMapFieldProps) => (
  <div className="flex gap-2">
    <TooltipWhenDisabled
      content={value?.description}
      disabled={!!value?.description}
      theme="dark"
    >
      <div>
        <ReadonlyItem
          className="w-[276px] overflow-hidden"
          title={I18n.t('evaluation_set_column')}
          typeText={getTypeText(value?.fieldSchema)}
          value={value?.target}
        />
      </div>
    </TooltipWhenDisabled>
    <EqualItem />
    <Select
      prefix={
        <Typography.Text
          ellipsis
          className="!coz-fg-secondary ml-3 !w-fit overflow-hidden"
        >
          {I18n.t('import_data_column')}
          {value?.fieldSchema?.isRequired ? (
            <span className="text-red ml-[2px]">*</span>
          ) : (
            ''
          )}
        </Typography.Text>
      }
      className="!w-[276px]"
      optionList={sourceColumns.map(column => ({
        label: column,
        value: column,
      }))}
      showClear
      value={value?.source}
      onChange={newTarget => {
        onChange?.({
          ...value,
          target: value?.target || '',
          source: newTarget as string,
        });
      }}
    ></Select>
  </div>
);
