// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useShallow } from 'zustand/react/shallow';
import { useUIStore, type BreadcrumbItemConfig } from '@cozeloop/stores';
import { useRouteInfo, useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { SwitchLang } from '@cozeloop/auth-pages';
import { Breadcrumb } from '@coze-arch/coze-design';

import { useMenuConfig } from '../navbar/menu-config';
import { getBreadcrumbMap } from './utils';

export function MainBreadcrumb() {
  const { app, subModule } = useRouteInfo();
  const { breadcrumbConfig, setBreadcrumbConfig } = useUIStore(
    useShallow(store => ({
      breadcrumbConfig: store.breadcrumbConfig,
      setBreadcrumbConfig: store.setBreadcrumbConfig,
    })),
  );

  const menuConfig = useMenuConfig();
  const [breadcrumbMap] = useState(getBreadcrumbMap(menuConfig));

  useEffect(() => {
    const config: BreadcrumbItemConfig[] = [];
    if (breadcrumbMap[app]) {
      config.push(breadcrumbMap[app]);
    }
    if (breadcrumbMap[`${app}/${subModule}`]) {
      config.push(breadcrumbMap[`${app}/${subModule}`]);
    }
    setBreadcrumbConfig(config);
  }, [app, subModule]);

  const navigate = useNavigateModule();

  const handleClick = (config: BreadcrumbItemConfig) => {
    navigate(`${config.path}`);
  };

  // 设置浏览器标签页标题
  useEffect(() => {
    const text = breadcrumbConfig
      .map(item => item.text)
      .filter(Boolean)
      .join(' - ');
    if (document.title !== text) {
      document.title = text;
    }
  }, [breadcrumbConfig]);

  return (
    <div className="h-[56px] flex items-center justify-between px-6 border-0 border-b border-solid coz-stroke-primary">
      <Breadcrumb
        separator={<div className="rotate-[22deg] coz-fg-dim">/</div>}
      >
        {breadcrumbConfig.map((c, index) => (
          <Breadcrumb.Item
            key={c.path || index}
            onClick={() => {
              if (index !== 0 && index !== breadcrumbConfig.length - 1) {
                handleClick(c);
              }
            }}
          >
            <span
              className={`!text-[13px] ${index === 0 ? 'cursor-default coz-fg-secondary' : ''}`}
            >
              {c.text}
            </span>
          </Breadcrumb.Item>
        ))}
      </Breadcrumb>
      <SwitchLang />
    </div>
  );
}
