// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { tag } from '@cozeloop/api-schema/data';

const { TagStatus } = tag;

export const formatTagDetailToFormValues = (tagDetail: tag.TagInfo) => ({
  ...tagDetail,
  tag_values:
    tagDetail.tag_values?.map(value => ({
      ...value,
      tag_status: value.status === TagStatus.Active,
    })) || [],
});