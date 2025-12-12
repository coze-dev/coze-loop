// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties } from 'react';

import classNames from 'classnames';
import { formateMsToSeconds, formatTimestampToString } from '@cozeloop/toolkit';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import { Tag, Typography } from '@coze-arch/coze-design';

export enum Status {
  Success = 'success',
  Failed = 'failed',
}

export interface QueryItemProps {
  debug_id?: Int64;
  status?: Status;
  createdTime?: number;
  systemPrompt?: string;
  duration?: Int64;
  className?: string;
  costMs?: Int64;
  style?: CSSProperties;
}

export const QueryItem = ({
  costMs,
  debug_id,
  status,
  createdTime,
  duration,
  className,
  style,
}: QueryItemProps) => (
  <div
    className={classNames('flex flex-col gap-2 max-w-[300px]', className)}
    style={style}
  >
    <Typography.Text
      ellipsis={{
        showTooltip: {
          opts: {
            theme: 'dark',
          },
        },
      }}
      strong
    >
      {debug_id}
    </Typography.Text>
    <div className="flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <Typography.Text type="tertiary">
          {I18n.t('prompt_time_consumed')}
        </Typography.Text>
        <Typography.Text>{formateMsToSeconds(costMs)}</Typography.Text>
        <Typography.Text type="tertiary">Tokens:</Typography.Text>
        <Typography.Text>{duration}</Typography.Text>
        {status === Status.Success ? (
          <Tag
            size="mini"
            color="green"
            prefixIcon={<IconCozCheckMarkCircleFill />}
          >
            {I18n.t('success')}
          </Tag>
        ) : (
          <Tag size="mini" color="red" prefixIcon={<IconCozCrossCircleFill />}>
            {I18n.t('failure')}
          </Tag>
        )}
      </div>
      <div className="flex gap-2">
        <Typography.Text type="tertiary">
          {I18n.t('prompt_request_start_time')}
        </Typography.Text>
        <Typography.Text
          ellipsis={{
            showTooltip: {
              opts: {
                content: formatTimestampToString(createdTime || 0),
              },
            },
          }}
          className="font-[500]"
        >
          {formatTimestampToString(createdTime || 0)}
        </Typography.Text>
      </div>
    </div>
  </div>
);
