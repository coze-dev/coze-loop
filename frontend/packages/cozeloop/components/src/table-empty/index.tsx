// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';
import {
  IconCozIllusNone,
  IconCozIllusNoneDark,
} from '@coze-arch/coze-design/illustrations';
import { EmptyState, type EmptyStateProps } from '@coze-arch/coze-design';

import { useI18n } from '../provider';

export function TableEmpty({ className, ...props }: EmptyStateProps) {
  const I18n = useI18n();
  return (
    <EmptyState
      className={classNames('my-10', className)}
      icon={<IconCozIllusNone />}
      darkModeIcon={<IconCozIllusNoneDark />}
      description={I18n.t('no_data')}
      {...props}
    />
  );
}
