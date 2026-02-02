// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, type ReactNode } from 'react';

import cls from 'classnames';
import { useI18nStore } from '@cozeloop/stores';
import { I18n } from '@cozeloop/i18n-adapter';

import s from './index.module.less';

interface Props {
  className?: string;
  brand?: ReactNode;
  children: ReactNode;
  classNames?: Partial<Record<'brand' | 'panel', string>>;
}

export function AuthFrame({ className, classNames, brand, children }: Props) {
  const lng = useI18nStore(state => state.lng);

  useEffect(() => {
    const title = I18n.t('platform_name');
    if (document.title !== title) {
      document.title = title;
    }
  }, [lng]);

  return (
    <div className={cls(s.frame, className)}>
      {brand ? (
        <div className={cls(s.brand, classNames?.brand)}>{brand}</div>
      ) : null}
      <div className={cls(s.panel, classNames?.panel)}>{children}</div>
    </div>
  );
}
