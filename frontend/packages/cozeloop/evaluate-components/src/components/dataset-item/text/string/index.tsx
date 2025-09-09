// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';

import { type DatasetItemProps } from '../../type';
import { PlainTextDatasetItem } from './plain-text';
import { MarkdownDatasetItem } from './markdown';
import { JSONDatasetItem } from './json';
import { CodeDatasetItem } from './code';

const DisplayFormatMap = {
  [FieldDisplayFormat.PlainText]: PlainTextDatasetItem,
  [FieldDisplayFormat.Code]: CodeDatasetItem,
  [FieldDisplayFormat.JSON]: JSONDatasetItem,
  [FieldDisplayFormat.Markdown]: MarkdownDatasetItem,
};

export const StringDatasetItem = (props: DatasetItemProps) => {
  const { displayFormat, fieldContent, fieldSchema } = props;
  const fieldFormat =
    fieldContent?.format ??
    fieldSchema?.default_display_format ??
    FieldDisplayFormat.PlainText;
  const format = displayFormat ? fieldFormat : FieldDisplayFormat.PlainText;
  const DisplayFormat = DisplayFormatMap[format] || PlainTextDatasetItem;
  return <DisplayFormat {...props} />;
};
