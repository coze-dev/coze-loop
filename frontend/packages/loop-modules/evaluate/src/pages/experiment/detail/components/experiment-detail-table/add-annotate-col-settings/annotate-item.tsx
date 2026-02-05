// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React, { type ReactNode } from 'react';

import classNames from 'classnames';
import { TAG_TYPE_TO_NAME_MAP } from '@cozeloop/tag-components';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { type tag } from '@cozeloop/api-schema/data';
import {
  CozAvatar,
  Typography,
  Tag,
  Space,
  Divider,
} from '@coze-arch/coze-design';

interface Props {
  data: tag.TagInfo;
  actions?: ReactNode;
  disabled?: boolean;
}

const AnnotateItem = ({ data, actions, disabled }: Props) => (
  <div className="flex items-center p-[10px] w-full cursor-default">
    <div className="flex-1 min-w-0">
      {/* 标签名称和分类 */}
      <Space className="mb-[7px] w-full">
        <div className="min-w-0">
          <TypographyText
            className={classNames(
              '!font-semibold',
              disabled ? '!coz-fg-dim' : '!coz-fg-primary',
            )}
          >
            {data.tag_key_name}
          </TypographyText>
        </div>
        {data.content_type ? (
          <Tag color="grey" className="shrink-0">
            {TAG_TYPE_TO_NAME_MAP[data.content_type]}
          </Tag>
        ) : null}
      </Space>

      <div className="flex w-full">
        {/* 标签描述 */}
        <div className="min-w-0 flex items-center">
          <TypographyText
            ellipsis={{ showTooltip: { opts: { theme: 'dark' } } }}
            className={classNames(
              'text-xs',
              disabled ? '!coz-fg-dim' : '!coz-fg-secondary',
            )}
          >
            {data.description}
          </TypographyText>
        </div>
        {data.description ? <Divider layout="vertical" margin={12} /> : null}

        {/* 更新人信息 */}
        <Space align="center" className="shrink-0">
          <Typography.Text
            type="secondary"
            className={classNames(
              'text-xs',
              disabled ? '!coz-fg-dim' : '!coz-fg-secondary',
            )}
          >
            {I18n.t('updated_by')}
          </Typography.Text>
          <CozAvatar
            size="small"
            src={data.base_info?.created_by?.avatar_url}
            className="w-5 h-5"
          />

          <Typography.Text
            className={classNames(
              'text-[13px]',
              disabled ? '!coz-fg-dim' : '!coz-fg-primary',
            )}
          >
            {data.base_info?.created_by?.name}
          </Typography.Text>
        </Space>
      </div>
    </div>
    {actions}
  </div>
);

export default AnnotateItem;
