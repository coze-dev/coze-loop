// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { type FieldSchema } from '@cozeloop/api-schema/evaluation';

import { ChipSelect } from '../common/chip-select';
import { getColumnType } from './util';
import { DISPLAY_FORMAT_MAP, DISPLAY_TYPE_MAP } from './type';
export const useItemFormat = (
  fieldSchema?: FieldSchema,
  datasetID?: string,
) => {
  const cacheKey = `${datasetID}-${fieldSchema?.key}`;
  const type = getColumnType(fieldSchema);
  const localStorageFormat = Number(localStorage.getItem(cacheKey));
  // 优先级： localstorage中的format>fieldschema中的format>兜底默认第一个类型
  const initformat =
    datasetID && DISPLAY_TYPE_MAP[type]?.includes(localStorageFormat)
      ? localStorageFormat
      : DISPLAY_TYPE_MAP[type]?.includes(fieldSchema?.default_display_format)
        ? fieldSchema?.default_display_format
        : DISPLAY_TYPE_MAP[type]?.[0];
  const [format, setFormat] = useState(initformat);
  const optionList =
    DISPLAY_TYPE_MAP[type]?.map(item => ({
      label: DISPLAY_FORMAT_MAP[item],
      value: item,
      chipColor: 'secondary',
    })) || [];
  const formatSelect =
    optionList?.length > 1 ? (
      <ChipSelect
        chipRender="selectedItem"
        value={format}
        size="small"
        onChange={value => {
          setFormat(value);
          localStorage.setItem(cacheKey, `${value}`);
        }}
        optionList={optionList}
      ></ChipSelect>
    ) : null;
  return {
    type,
    format,
    formatSelect,
  };
};
