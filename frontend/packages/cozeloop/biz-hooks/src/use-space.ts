// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useParams } from 'react-router-dom';

import { useSpaceStore } from '@cozeloop/account';

import DemoSpaceIcon from './assets/demo-space-icon.svg';

/** 获取空间信息 */
export function useSpace() {
  const space = useSpaceStore(s => s.space);
  const spaces = useSpaceStore(s => s.spaces);
  const { spaceID = '' } = useParams();

  return {
    space: {
      id: space?.id,
      name: space?.name,
      icon_url: DemoSpaceIcon,
    },
    spaceID: space?.id ?? spaceID,
    spaceIDWhenDemoSpaceItsPersonal: space?.id ?? spaceID,
    spaceList: spaces.map(it => ({ ...it })),
    inited: true,
    getDefaultSpaceID: () => spaceID,
  };
}
