// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ModifyUserProfileRequest,
  type ResetPasswordRequest,
} from '@cozeloop/api-schema/foundation';
import { FoundationApi } from '@cozeloop/api-schema';

/** 用户服务 */
export const userService = (() => ({
  async register(email: string, password: string) {
    if (!email || !password) {
      throw new Error('Invalid email or password');
    }

    const resp = await FoundationApi.Register({
      email,
      password,
    });

    return resp;
  },
  async login(email: string, password: string) {
    if (!email || !password) {
      throw new Error('Invalid email or password');
    }

    const resp = await FoundationApi.LoginByPassword({
      email,
      password,
    });

    return resp;
  },
  async logout(token?: string) {
    await FoundationApi.Logout({ token });
  },
  async modifyUserProfile(req: ModifyUserProfileRequest) {
    const resp = await FoundationApi.ModifyUserProfile(req);

    return resp.user_info;
  },
  async resetPassword(req: ResetPasswordRequest) {
    await FoundationApi.ResetPassword(req);
  },
  async getUserInfo(disableErrorToast?: boolean) {
    const resp = await FoundationApi.GetUserInfoByToken(
      {},
      { disableErrorToast },
    );

    return resp.user_info;
  },
}))();
