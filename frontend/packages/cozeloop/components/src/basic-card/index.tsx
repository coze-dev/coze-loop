import { type ReactNode } from 'react';

import classNames from 'classnames';

export interface CardProps {
  title: ReactNode;
  children?: ReactNode;
  className?: string;
}

export const BasicCard = ({ title, children, className }: CardProps) => (
  <div
    className={classNames(
      'border border-solid coz-stroke-plus rounded-[6px] overflow-hidden',
      className,
    )}
  >
    <div className="text-sm font-medium px-4 h-[44px] flex items-center coze-fg-primary bg-[#F7F7FC]">
      {title}
    </div>
    {children ? <div className="p-4 bg-white">{children}</div> : null}
  </div>
);
