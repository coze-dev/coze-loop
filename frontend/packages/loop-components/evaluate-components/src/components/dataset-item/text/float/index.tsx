// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../../type';
import { FloatDatasetItemReadOnly } from './readonly';
import { FloatDatasetItemEdit } from './edit';

export const FloatDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <FloatDatasetItemEdit {...props} />
  ) : (
    <FloatDatasetItemReadOnly {...props} />
  );
