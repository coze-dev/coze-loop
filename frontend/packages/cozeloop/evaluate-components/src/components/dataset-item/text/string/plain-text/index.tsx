// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../../../type';
import { PlainTextDatasetItemReadOnly } from './readonly';
import { PlainTextDatasetItemEdit } from './edit';

export const PlainTextDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <PlainTextDatasetItemEdit {...props} />
  ) : (
    <PlainTextDatasetItemReadOnly {...props} />
  );
