// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../type';
import { MultipartDatasetItemReadOnly } from './readonly';
import { MultipartDatasetItemEdit } from './edit';

export const MultipartDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <MultipartDatasetItemEdit {...props} />
  ) : (
    <MultipartDatasetItemReadOnly {...props} />
  );
