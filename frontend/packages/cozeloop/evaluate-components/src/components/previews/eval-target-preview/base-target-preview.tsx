// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { JumpIconButton } from '@cozeloop/components';
import { Tag, Tooltip } from '@coze-arch/coze-design';

import { TypographyText } from '../../text-ellipsis';

export default function BaseTargetPreview({
  name,
  version,
  showVersion = true,
  enableLinkJump,
  className,
  onClick,
}: {
  name: React.ReactNode;
  version?: string;
  showVersion?: boolean;
  enableLinkJump?: boolean;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
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
      <TypographyText>{name ?? '-'}</TypographyText>
      {showVersion ? (
        <Tag size="small" color="primary" className="shrink-0">
          {version ?? '-'}
        </Tag>
      ) : null}
      {enableLinkJump ? (
        <Tooltip theme="dark" content={I18n.t('view_detail')}>
          <div>
            <JumpIconButton className="hidden group-hover:flex" />
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}
