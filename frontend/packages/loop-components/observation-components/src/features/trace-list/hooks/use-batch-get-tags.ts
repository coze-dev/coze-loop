// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useRequest } from 'ahooks';
import { type BatchGetTagsResponse } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

export const useBatchGetTags = (
  customParams: Record<string, any>,
): ReturnType<typeof useRequest<BatchGetTagsResponse, [string[]]>> => {
  const service = useRequest(
    async (tagKeyIds: string[]) => {
      const result = await DataApi.BatchGetTags({
        workspace_id: customParams?.spaceID ?? '',
        tag_key_ids: tagKeyIds,
      });
      return result;
    },
    {
      manual: true,
    },
  );
  return service;
};
