// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { DataApi } from '@cozeloop/api-schema';

export const useGetTagSpec = () => {
  const { spaceID } = useSpace();

  const service = useRequest(
    async () =>
      await DataApi.GetTagSpec({
        workspace_id: spaceID,
      }),
  );

  return service;
};
