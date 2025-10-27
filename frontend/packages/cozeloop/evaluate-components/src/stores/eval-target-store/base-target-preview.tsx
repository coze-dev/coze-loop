// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
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
        <Tooltip theme="dark" content={I18n.t('view_detail')}>
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
