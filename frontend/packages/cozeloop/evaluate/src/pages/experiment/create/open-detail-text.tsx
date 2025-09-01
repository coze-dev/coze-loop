import classNames from 'classnames';
import { Tooltip } from '@coze-arch/coze-design';

export function OpenDetailText({
  className,
  text,
  url,
}: {
  url: string;
  className?: string;
  text?: string;
}) {
  return (
    <Tooltip theme="dark" content="查看详情">
      <div
        className={classNames(
          'flex-shrink-0 text-sm text-brand-9 font-normal cursor-pointer !p-[2px] ',
          className,
        )}
        onClick={e => {
          e.stopPropagation();
          window.open(url);
        }}
      >
        {text || '查看详情'}
      </div>
    </Tooltip>
  );
}
