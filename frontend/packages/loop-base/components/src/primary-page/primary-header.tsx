// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type { ReactNode } from 'react';

import classNames from 'classnames';

interface PrimaryPageHeaderProps {
  pageTitle?: ReactNode;
  filterSlot?: ReactNode;
  children?: ReactNode;
  headerClassName?: string;
  titleClassName?: string;
  contentClassName?: string;
  className?: string;
  titleSlot?: ReactNode;
}

export const PrimaryPage = ({
  pageTitle,
  filterSlot,
  children,
  contentClassName,
  className,
  headerClassName,
  titleClassName,
  titleSlot,
}: PrimaryPageHeaderProps) => (
  <div
    className={classNames(
      'pt-2 pb-3 h-full max-h-full flex flex-col',
      className,
    )}
  >
    {pageTitle || titleSlot ? (
      <div
        className={classNames(
          'flex items-center justify-between py-4 px-6',
          headerClassName,
        )}
      >
        <div
          className={classNames(
            'text-[20px] font-medium leading-6 coz-fg-plus',
            titleClassName,
          )}
        >
          {pageTitle}
        </div>
        <div>{titleSlot}</div>
      </div>
    ) : null}
    {filterSlot ? (
      <div className="box-border coz-fg-secondary pt-1 pb-3 px-6">
        {filterSlot}
      </div>
    ) : null}
    <div
      className={classNames(
        'flex-1 h-full max-h-full overflow-hidden px-6',
        contentClassName,
      )}
    >
      {children}
    </div>
  </div>
);
