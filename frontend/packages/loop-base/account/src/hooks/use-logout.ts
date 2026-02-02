// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRequest } from 'ahooks';

import { useUserStore } from '../stores/user-store';
import { userService } from '../services/user-service';

export function useLogout() {
  const patch = useUserStore(s => s.patch);

  return useRequest(() => userService.logout(), {
    manual: true,
    onSuccess: () => patch({ userInfo: undefined }),
  });
}
