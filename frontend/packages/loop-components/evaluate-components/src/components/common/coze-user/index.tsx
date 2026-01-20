// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { UserProfile } from '@cozeloop/components';
import { type UserInfo } from '@cozeloop/api-schema/evaluation';

export function CozeUser({
  user,
  size = 'nomal',
}: {
  user: UserInfo | undefined;
  size?: 'small' | 'nomal';
}) {
  const avatarClassName = size === 'small' ? '!w-[18px] !h-[18px]' : '';
  const className = size === 'small' ? 'gap-1' : '';
  return (
    <UserProfile
      avatarUrl={user?.avatar_url}
      name={user?.name}
      className={className}
      avatarClassName={avatarClassName}
    />
  );
}
