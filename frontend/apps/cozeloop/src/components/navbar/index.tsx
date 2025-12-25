// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';

import classNames from 'classnames';
import { useHover } from 'ahooks';
import { useNavbarCollapsed } from '@cozeloop/hooks';
import { useRouteInfo, useNavigateModule } from '@cozeloop/biz-hooks-adapter';
import { IconCozSideNav } from '@coze-arch/coze-design/icons';
import { Nav, Divider } from '@coze-arch/coze-design';

import logo from '@/assets/images/logo.svg';
import logoMini from '@/assets/images/coze.svg';

import { UserInfoSection } from '../user-info-section';
import { NavbarList } from './navbar-list';
import { useMenuConfig } from './menu-config';
import { FooterMenus } from './footer-menus';

import styles from './index.module.less';

export function Navbar() {
  const navigate = useNavigateModule();
  const { isCollapsed, toggleCollapsed } = useNavbarCollapsed();
  const { app, subModule } = useRouteInfo();
  /** 选中的导航栏 */
  const [selectedKeys, setSelectedKeys] = useState<string[]>(() => []);
  const menuConfig = useMenuConfig();
  const navRef = useRef<HTMLDivElement>(null);
  const isHovered = useHover(navRef);

  useEffect(() => {
    if (app && subModule) {
      setSelectedKeys([`${app}/${subModule}`]);
    }
  }, [app, subModule]);

  const handleSelect = (path: string) => {
    if (!path.startsWith('actions/')) {
      if (path.startsWith('enterprise-manage') || path.startsWith('open')) {
        navigate(path, {
          params: {
            spaceID: undefined,
          },
        });
        return;
      } else {
        navigate(path);
      }
    }
  };

  return (
    <div className="h-full" ref={navRef}>
      <Nav
        isCollapsed={isCollapsed}
        className={classNames(
          'h-full min-h-full max-h-full min-w-[88px] !px-0 overflow-hidden !bg-white',
          styles.navbar,
        )}
        selectedKeys={selectedKeys}
        onSelect={data => {
          handleSelect(`${data.itemKey || ''}`);
        }}
      >
        <div className="px-6 mb-[10px]">
          <Nav.Header className="flex items-center w-full gap-1 !pt-[1px] !pb-[17px] !pl-[8px] !pr-0 justify-between">
            {isCollapsed ? (
              <img src={logoMini} className="w-[26px] h-[26px]" />
            ) : (
              <img src={logo} className="h-[26px] " />
            )}
            <IconCozSideNav
              className="cursor-pointer flex-shrink-0 coz-fg-secondary h-[14px] w-[14px]"
              onClick={toggleCollapsed}
            />
          </Nav.Header>
        </div>
        <NavbarList
          menus={menuConfig}
          isCollapsed={isCollapsed}
          selectedKeys={selectedKeys}
          className="px-6 flex-1 !pr-[18px] pb-2"
        />
        <Divider className="relative" />
        <div className="pt-4 pb-3 px-6 relative">
          <FooterMenus isCollapsed={isCollapsed} isHovered={isHovered} />
          <UserInfoSection isCollapsed={isCollapsed} />
        </div>
      </Nav>
    </div>
  );
}
