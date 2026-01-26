// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { Typography } from '@coze-arch/coze-design';

export function DevLayout({
  title,
  actionBtns,
  children,
  className,
  style,
  wrapperClassName,
  domRef,
}: {
  title: React.ReactNode;
  actionBtns?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  wrapperClassName?: string;
  style?: React.CSSProperties;
  domRef?: React.Ref<HTMLDivElement>;
}) {
  return (
    <div
      className={classNames('flex flex-col h-full w-full', className)}
      style={style}
      ref={domRef}
    >
      <div
        className={classNames(
          'h-[40px] px-6 py-2 box-border coz-fg-plus w-full border-0 border-t border-b border-solid flex justify-between items-center',
          wrapperClassName,
        )}
        style={{ background: '#FCFCFF' }}
      >
        {typeof title === 'string' ? (
          <Typography.Text strong>{title}</Typography.Text>
        ) : (
          title
        )}
        {actionBtns}
      </div>
      {children}
    </div>
  );
}
