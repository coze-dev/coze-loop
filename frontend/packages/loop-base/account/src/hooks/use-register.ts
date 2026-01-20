// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemoizedFn } from 'ahooks';

import { useUserStore } from '../stores/user-store';
import { userService } from '../services/user-service';

export function useRegister() {
  const patch = useUserStore(s => s.patch);

  const register = useMemoizedFn(async (email: string, password: string) => {
    try {
      patch({ settling: true });
      const resp = await userService.register(email, password);

      resp.user_info
        ? patch({ userInfo: resp.user_info, settling: false })
        : patch({ settling: false });
    } catch (e) {
      console.error(e);
      patch({ settling: false });
    }
  });

  return register;
}
