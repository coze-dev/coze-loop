// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useShallow } from 'zustand/react/shallow';
import { useUIStore } from '@cozeloop/stores';

export function useNavbarCollapsed() {
  const { navbarCollapsed, setNavbarCollapsed, toggleNavbarCollapsed } =
    useUIStore(
      useShallow(state => ({
        navbarCollapsed: state.navbarCollapsed,
        setNavbarCollapsed: state.setNavbarCollapsed,
        toggleNavbarCollapsed: state.toggleNavbarCollapsed,
      })),
    );

  return {
    isCollapsed: navbarCollapsed,
    setCollapsed: setNavbarCollapsed,
    toggleCollapsed: toggleNavbarCollapsed,
  };
}
