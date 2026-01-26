// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type UpdatePersonalAccessTokenRequest,
  type CreatePersonalAccessTokenRequest,
} from '@cozeloop/api-schema/foundation';
import { FoundationApi } from '@cozeloop/api-schema';

/** 鉴权服务 */
export const authnService = (() => ({
  /**
   * list pat
   *
   * @param pageNum - page number, starts from 1, default to `1`
   * @param pageSize - page size, default to `20`
   */
  async listPat(pageNum = 1, pageSize = 20) {
    const resp = await FoundationApi.ListPersonalAccessToken({
      page_number: pageNum,
      page_size: pageSize,
    });

    return resp.personal_access_tokens;
  },
  async getPat(id: string) {
    const resp = await FoundationApi.GetPersonalAccessToken({ id });

    return resp.personal_access_token;
  },
  async createPat(req: CreatePersonalAccessTokenRequest) {
    const resp = await FoundationApi.CreatePersonalAccessToken(req);

    return {
      pat: resp.personal_access_token,
      token: resp.token,
    };
  },
  async updatePat(req: UpdatePersonalAccessTokenRequest) {
    await FoundationApi.UpdatePersonalAccessToken(req);
  },
  async deletePat(id: string) {
    await FoundationApi.DeletePersonalAccessToken({ id });
  },
}))();
