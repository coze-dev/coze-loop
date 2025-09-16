// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type DatasetItemProps } from '../../type';
import { ObjectDatasetItemReadOnly } from './readonly';
import { ObjectDatasetItemEdit } from './edit';

export const ObjectDatasetItem = (props: DatasetItemProps) =>
  props.isEdit ? (
    <ObjectDatasetItemEdit {...props} />
  ) : (
    <ObjectDatasetItemReadOnly {...props} />
  );
