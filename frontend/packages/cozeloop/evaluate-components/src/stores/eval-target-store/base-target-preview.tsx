import classNames from 'classnames';
import { JumpIconButton } from '@cozeloop/components';
import { type Ellipsis, Tag, Tooltip } from '@coze-arch/coze-design';

import { TypographyText } from '../../components/text-ellipsis';

export function BaseTargetPreview({
  name,
  version,
  showVersion = true,
  enableLinkJump,
  className,
  jumpBtnClassName,
  onClick,
  nameEllipsis = {},
}: {
  name: React.ReactNode;
  version?: string;
  showVersion?: boolean;
  enableLinkJump?: boolean;
  className?: string;
  jumpBtnClassName?: string;
  onClick?: (e: React.MouseEvent) => void;
  nameEllipsis?: Ellipsis;
}) {
  return (
    <div
      className={classNames(
        'group inline-flex items-center gap-1 overflow-hidden cursor-pointer max-w-[100%]',
        className,
      )}
      onClick={e => {
        if (!enableLinkJump) {
          return;
        }
        e.stopPropagation();
        onClick?.(e);
      }}
    >
      <TypographyText ellipsis={nameEllipsis}>{name ?? '-'}</TypographyText>
      {showVersion ? (
        <Tag size="small" color="primary" className="shrink-0">
          {version ?? '-'}
        </Tag>
      ) : null}
      {enableLinkJump ? (
        <Tooltip theme="dark" content="查看详情">
          <div>
            <JumpIconButton
              className={classNames(
                'hidden group-hover:flex',
                jumpBtnClassName,
              )}
            />
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}
