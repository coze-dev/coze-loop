// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties } from 'react';

import classNames from 'classnames';
import { IconCozCopy } from '@coze-arch/coze-design/icons';
import { IconButton, Tooltip, Typography } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { handleCopy } from '../utils/basic';

interface TextWithCopyProps {
  content?: string;
  displayText?: string;
  copyTooltipText?: string;
  maxWidth?: number | string;
  className?: string;
  style?: CSSProperties;
  textClassName?: string;
  textType?:
    | 'success'
    | 'secondary'
    | 'primary'
    | 'danger'
    | 'warning'
    | 'tertiary'
    | 'quaternary';
  onlyIconCopy?: boolean;
}

export function TextWithCopy({
  displayText,
  copyTooltipText,
  content,
  className,
  maxWidth,
  style,
  textClassName,
  textType = 'secondary',
  onlyIconCopy,
}: TextWithCopyProps) {
  const I18n = useI18n();
  return (
    <div
      className={classNames(
        'flex items-baseline justify-start gap-1',
        className,
      )}
      style={style}
    >
      <Typography.Text
        className={classNames(
          'max-w-full',
          {
            'cursor-pointer': !onlyIconCopy,
          },
          textClassName,
        )}
        type={textType}
        style={{ maxWidth }}
        ellipsis={{
          showTooltip: { opts: { theme: 'dark', content } },
        }}
        onClick={
          onlyIconCopy
            ? undefined
            : e => {
                content && handleCopy(content);
                e?.stopPropagation();
              }
        }
      >
        {displayText || content || ''}
      </Typography.Text>
      {content ? (
        <Tooltip
          content={copyTooltipText || I18n.t('copy_content')}
          theme="dark"
        >
          <IconButton
            size="mini"
            color="secondary"
            className="flex-shrink-0"
            style={{
              width: 21,
              height: 21,
            }}
            icon={
              <IconCozCopy
                onClick={e => {
                  content && handleCopy(content);
                  e?.stopPropagation();
                }}
                fill="var(--semi-color-text-2)"
              />
            }
          />
        </Tooltip>
      ) : null}
    </div>
  );
}
