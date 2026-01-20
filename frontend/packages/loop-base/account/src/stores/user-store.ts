// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { create } from 'zustand';
import { type UserInfoDetail } from '@cozeloop/api-schema/foundation';

interface UserState {
  userInfo?: UserInfoDetail;
  settling?: boolean;
}

interface UserAction {
  patch: (s: Partial<UserState>) => void;
  reset: () => void;
}

export const useUserStore = create<UserState & UserAction>((set, get) => ({
  patch: (s: Partial<UserState>) => {
    if (Object.keys(s).length) {
      set({ ...s });
    }
  },
  reset: () => set({ userInfo: undefined, settling: undefined }),
}));

export function setUserInfo(userInfo?: UserInfoDetail) {
  if (!userInfo) {
    return;
  }

  useUserStore.getState().patch({ userInfo });
}
