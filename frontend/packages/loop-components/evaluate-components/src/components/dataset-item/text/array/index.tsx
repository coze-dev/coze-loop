// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../../type';
import { ArrayDatasetItemReadOnly } from './readonly';
import { ArrayDatasetItemEdit } from './edit';

export const ArrayDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <ArrayDatasetItemEdit {...props} />
  ) : (
    <ArrayDatasetItemReadOnly {...props} />
  );
