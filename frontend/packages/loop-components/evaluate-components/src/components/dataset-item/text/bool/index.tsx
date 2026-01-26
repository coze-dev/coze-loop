// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../../type';
import { BoolDatasetItemReadOnly } from './readonly';
import { BoolDatasetItemEdit } from './edit';

export const BoolDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <BoolDatasetItemEdit {...props} />
  ) : (
    <BoolDatasetItemReadOnly {...props} />
  );
