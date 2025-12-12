import classNames from 'classnames';
import { Typography } from '@coze-arch/coze-design';

interface PromptDevLayoutProps
  extends Omit<React.HTMLAttributes<HTMLDivElement>, 'title'> {
  title: React.ReactNode;
  actionBtns?: React.ReactNode;
  wrapperClassName?: string;
}

export function PromptDevLayout({
  title,
  actionBtns,
  children,
  className,
  wrapperClassName,
  ...rest
}: PromptDevLayoutProps) {
  return (
    <div
      className={classNames('flex flex-col h-full w-full', className)}
      {...rest}
    >
      <div
        className={classNames(
          'h-[42px] px-6 box-border coz-fg-plus w-full border-0 border-t border-solid flex justify-between items-center flex-shrink-0',
          wrapperClassName,
        )}
      >
        {typeof title === 'string' ? (
          <Typography.Title heading={6}>{title}</Typography.Title>
        ) : (
          title
        )}
        {actionBtns}
      </div>
      {children}
    </div>
  );
}
