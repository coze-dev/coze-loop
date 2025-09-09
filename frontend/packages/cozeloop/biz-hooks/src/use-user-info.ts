// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useUserStore } from '@cozeloop/account';

export function useUserInfo() {
  const userInfo = useUserStore(s => s.userInfo);

  return {
    app_id: 1,
    user_id_str: userInfo?.user_id,
    email: userInfo?.email,
    screen_name: userInfo?.nick_name,
    name: userInfo?.name,
    avatar_url: userInfo?.avatar_url || '',
  };
}
