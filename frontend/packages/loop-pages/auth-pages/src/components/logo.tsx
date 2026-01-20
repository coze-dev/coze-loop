// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { ReactComponent as LogoIcon } from '@/assets/logo.svg';

interface Props {
  className?: string;
}

export function Logo({ className }: Props) {
  return (
    <div className={className}>
      <LogoIcon />
    </div>
  );
}
