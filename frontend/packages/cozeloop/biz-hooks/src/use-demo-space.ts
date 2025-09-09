// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSpace } from './use-space';
import { DEMO_SPACE_ID } from './constants';

/**
 * isDemoSpace 标识当前空间是否为 demo 空间，且没有管理员权限，用于通用判断
 * isDemoSpaceID 判断当前空间是否为 demo 空间
 * isDemoSpaceAdmin 判断传入的空间是否为 Demo 空间，并且是管理员权限
 * @return
 */
export function useDemoSpace() {
  const { spaceID } = useSpace();

  return {
    demoSpaceID: DEMO_SPACE_ID,
    isDemoSpace: false,
    isDemoSpaceID: DEMO_SPACE_ID === spaceID,
    checkIsDemoSpace: (id: string) => DEMO_SPACE_ID === id,
    isDemoSpaceAdmin: (_id: string) => false,
    isDemoSpaceVisitor: (_id: string) => true,
  };
}
