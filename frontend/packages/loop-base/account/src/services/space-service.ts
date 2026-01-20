// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ListUserSpaceRequest } from '@cozeloop/api-schema/foundation';
import { FoundationApi } from '@cozeloop/api-schema';

export const spaceService = (() => ({
  async getSpace(spaceId: number, appId?: number) {
    const resp = await FoundationApi.GetSpace({
      space_id: spaceId,
    });

    return resp.space;
  },
  listSpaces(req?: ListUserSpaceRequest) {
    return FoundationApi.ListUserSpaces({
      page_number: 1,
      page_size: 100,
      ...req,
    });
  },
}))();
