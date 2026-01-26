// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Result } from 'ahooks/lib/useRequest/src/types';
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type BatchUpdateTagStatusRequest } from '@cozeloop/api-schema/data';
import { DataApi } from '@cozeloop/api-schema';

interface UpdateTagStatusParams {
  tagKeyIds: string[];
  toStatus: BatchUpdateTagStatusRequest['to_status'];
}

export const useUpdateTagStatus = (): Result<void, [UpdateTagStatusParams]> => {
  const { spaceID } = useSpace();

  return useRequest(
    async ({ tagKeyIds, toStatus }: UpdateTagStatusParams) => {
      await DataApi.BatchUpdateTagStatus({
        workspace_id: spaceID,
        tag_key_ids: tagKeyIds,
        to_status: toStatus,
      });
    },
    {
      manual: true,
    },
  );
};
