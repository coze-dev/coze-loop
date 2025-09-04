// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type Space } from '@cozeloop/api-schema/foundation';
import {
  IconCozChat,
  IconCozDatabase,
  IconCozPlayCircle,
  IconCozDashboard,
  IconCozLightbulb,
  IconCozNode,
  IconCozChatFill,
  IconCozPlayCircleFill,
  IconCozDashboardFill,
  IconCozDatabaseFill,
  IconCozLightbulbFill,
  IconCozNodeFill,
  IconCozTag,
  IconCozTagFill,
} from '@coze-arch/coze-design/icons';

export interface MenuConfig {
  itemKey: string;
  text: string;
  icon?: ReactNode;
  selectedIcon?: ReactNode;
  items?: MenuConfig[];
  /** 在侧边栏隐藏 */
  hideInNavbar?: boolean;
  /** 打开新链接 */
  link?: string;
  /** 禁用链接 */
  disabled?: boolean;
  /** 是否可见 */
  visible?: (data: { space?: Space }) => boolean;
}

export function useMenuConfig() {
  const menuConfig: MenuConfig[] = [
    {
      itemKey: 'pe',
      text: I18n.t('prompt_engineering'),
      visible: ({ space }) => Boolean(space?.id),
      items: [
        {
          itemKey: 'pe/prompts',
          text: I18n.t('prompt_development'),
          icon: <IconCozChat />,
          selectedIcon: <IconCozChatFill className="coz-fg-plus" />,
        },
        {
          itemKey: 'pe/playground',
          text: 'Playground',
          icon: <IconCozPlayCircle />,
          selectedIcon: <IconCozPlayCircleFill className="coz-fg-plus" />,
        },
      ],
    },
    {
      itemKey: 'evaluation',
      text: I18n.t('evaluation'),
      visible: ({ space }) => Boolean(space?.id),
      items: [
        {
          itemKey: 'evaluation/datasets',
          text: I18n.t('evaluation_set'),
          icon: <IconCozDatabase />,
          selectedIcon: <IconCozDatabaseFill className="coz-fg-plus" />,
        },
        {
          itemKey: 'evaluation/evaluators',
          text: I18n.t('evaluator'),
          icon: <IconCozLightbulb />,
          selectedIcon: <IconCozLightbulbFill className="coz-fg-plus" />,
        },
        {
          itemKey: 'evaluation/experiments',
          text: I18n.t('experiment'),
          icon: <IconCozDashboard />,
          selectedIcon: <IconCozDashboardFill className="coz-fg-plus" />,
        },
      ],
    },
    {
      itemKey: 'observation',
      text: I18n.t('observation'),
      visible: ({ space }) => Boolean(space?.id),
      items: [
        {
          itemKey: 'observation/traces',
          text: 'Trace',
          icon: <IconCozNode />,
          selectedIcon: <IconCozNodeFill className="coz-fg-plus" />,
        },
      ],
    },
    {
      itemKey: 'tag',
      text: I18n.t('tag'),
      visible: ({ space }) => Boolean(space?.id),
      items: [
        {
          itemKey: 'tag/tag',
          text: I18n.t('tag_management'),
          icon: <IconCozTag />,
          selectedIcon: <IconCozTagFill className="coz-fg-plus" />,
        },
      ],
    },
  ];

  return menuConfig;
}
