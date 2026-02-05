// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { devtools } from 'zustand/middleware';
import { create } from 'zustand';
import { produce } from 'immer';
import EventEmitter from 'eventemitter3';

import { storage } from '../utils/storage';

export interface BreadcrumbItemConfig {
  text: string;
  path?: string;
}
interface UIState {
  publish$: EventEmitter;
  /** 面包屑配置 */
  breadcrumbConfig: BreadcrumbItemConfig[];
  /** 导航栏是否折叠 */
  navbarCollapsed: boolean;
}

interface UIAction {
  /** 设置面包屑配置 */
  setBreadcrumbConfig: (config: BreadcrumbItemConfig[]) => void;
  pushBreadcrumb: (config: BreadcrumbItemConfig) => void;
  popBreadcrumb: () => void;
  setNavbarCollapsed: (collapsed: boolean) => void;
  toggleNavbarCollapsed: () => void;
}

export enum UIEvent {
  // 打开订阅弹窗
  OPEN_SUBSCRIPTION_MODAL = 'open-subscription-modal',
}

const IS_DEV_MODE = process.env.NODE_ENV === 'development';

export const useUIStore = create<UIState & UIAction>()(
  devtools(
    (set, get) => {
      const publish$ = new EventEmitter();
      return {
        publish$,
        breadcrumbConfig: [],
        navbarCollapsed: Number(storage.getItem('navbar-collapsed') ?? 0) === 1,
        setBreadcrumbConfig: config => {
          set({ breadcrumbConfig: config });
        },
        pushBreadcrumb: config => {
          set(
            produce<UIState>(state => {
              state.breadcrumbConfig.push(config);
            }),
          );
        },
        popBreadcrumb: () => {
          set(
            produce<UIState>(state => {
              state.breadcrumbConfig.pop();
            }),
          );
        },
        setNavbarCollapsed: collapsed => {
          storage.setItem('navbar-collapsed', String(collapsed ? 1 : 0));
          set({ navbarCollapsed: collapsed });
        },
        toggleNavbarCollapsed: () => {
          set(state => {
            storage.setItem(
              'navbar-collapsed',
              String(state.navbarCollapsed ? 0 : 1),
            );
            return { navbarCollapsed: !state.navbarCollapsed };
          });
        },
      };
    },
    { name: 'cozeloop.ui', enabled: IS_DEV_MODE },
  ),
);
