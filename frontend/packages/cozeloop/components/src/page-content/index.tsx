// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties, type ReactNode } from 'react';

import classNames from 'classnames';
import {
  IconCozIllus404Dark,
  IconCozIllus404,
  IconCozIllusErrorDark,
  IconCozIllusError,
  IconCozIllusLock,
  IconCozIllusLockDark,
  IconCozIllusEmpty,
  IconCozIllusEmptyDark,
} from '@coze-arch/coze-design/illustrations';
import { Empty, Spin, type EmptyStateProps } from '@coze-arch/coze-design';

interface PageLoadingProps {
  tip?: ReactNode;
  className?: string;
  style?: CSSProperties;
}
interface FullPageProps {
  className?: string;
  style?: CSSProperties;
  children?: ReactNode;
}

type PageContentProps = Omit<EmptyStateProps, 'image' | 'darkModeImage'>;

export function FullPage({ children, className, style }: FullPageProps) {
  return (
    <div
      className={classNames(
        'w-full h-full flex items-center justify-center bg-semi-bg-1 z-10',
        className,
      )}
      style={style}
    >
      {children}
    </div>
  );
}
export function PageLoading(props: PageLoadingProps) {
  return (
    <FullPage className={props.className} style={props.style}>
      <Spin wrapperClassName="w-full h-full" spinning tip={props.tip} />
    </FullPage>
  );
}

export function PageNotFound({ className, ...props }: PageContentProps) {
  return (
    <FullPage className={className}>
      <Empty
        image={<IconCozIllus404 className="text-[160px]" />}
        darkModeImage={<IconCozIllus404Dark className="text-[160px]" />}
        description="页面丢失了"
        {...props}
      />
    </FullPage>
  );
}

export function PageError({ className, ...props }: PageContentProps) {
  return (
    <FullPage className={className}>
      <Empty
        image={<IconCozIllusError className="text-[160px]" />}
        darkModeImage={<IconCozIllusErrorDark className="text-[160px]" />}
        description="页面加载失败"
        {...props}
      />
    </FullPage>
  );
}

export function PageNoAuth({ className, ...props }: PageContentProps) {
  return (
    <FullPage className={className}>
      <Empty
        image={<IconCozIllusLock className="text-[160px]" />}
        darkModeImage={<IconCozIllusLockDark className="text-[160px]" />}
        description="暂无权限"
        {...props}
      ></Empty>
    </FullPage>
  );
}

export function PageNoContent({ className, ...props }: PageContentProps) {
  return (
    <FullPage className={className}>
      <Empty
        image={<IconCozIllusEmpty className="text-[160px]" />}
        darkModeImage={<IconCozIllusEmptyDark className="text-[160px]" />}
        {...props}
      />
    </FullPage>
  );
}
