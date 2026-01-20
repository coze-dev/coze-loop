// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type TreeNodeData } from '@coze-arch/coze-design';

export function updateTreeData(
  list: TreeNodeData[],
  key: string,
  children?: TreeNodeData[],
) {
  const newChildren: TreeNodeData[] = list.map(node => {
    if (node.key === key) {
      return { ...node, children };
    }
    if (node.children) {
      return {
        ...node,
        children: updateTreeData(node.children, key, children),
      };
    }
    return node;
  });
  return newChildren;
}
