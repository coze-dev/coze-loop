// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cn from 'classnames';
import { CozAvatar, Typography } from '@coze-arch/coze-design';

interface UserInfoProps {
  avatarUrl?: string;
  name?: string;
  className?: string;
  avatarClassName?: string;
  userNameClassName?: string;
}
export const UserProfile = ({
  avatarUrl,
  name,
  className,
  avatarClassName,
  userNameClassName,
}: UserInfoProps) => (
  <div className={cn('flex items-center gap-[6px] w-full', className)}>
    <CozAvatar
      className={cn('!w-[20px] !h-[20px]', avatarClassName)}
      src={avatarUrl}
    >
      {name}
    </CozAvatar>
    <Typography.Text
      className={cn('flex-1 overflow-hidden !text-[13px]', userNameClassName)}
      style={{
        fontSize: 'inherit',
        color: 'inherit',
        fontWeight: 'inherit',
        lineHeight: 'inherit',
      }}
      ellipsis={{
        showTooltip: {
          opts: {
            theme: 'dark',
          },
        },
      }}
    >
      {name}
    </Typography.Text>
  </div>
);
