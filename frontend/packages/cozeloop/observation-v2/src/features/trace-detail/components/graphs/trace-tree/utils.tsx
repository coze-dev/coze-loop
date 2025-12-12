import { SpanStatus } from '@cozeloop/api-schema/observation';

import {
  BROKEN_ROOT_SPAN_ID,
  NORMAL_BROKEN_SPAN_ID,
} from '@/features/trace-detail/constants/span';

import { type TreeNode, PathEnum } from '../tree/typing';
import { type SpanNode } from './type';
import { CustomTreeNode } from './node';
import { spanStatusConfig } from './config';

/** 链式节点转化为树节点 */
export const spanNode2TreeNode = ({
  spanNode,
  onCollapseChange,
  matchedSpanIds,
  parentPath = [],
  isFirstLevel = true,
  isLastChild = false,
}: {
  spanNode: SpanNode;
  onCollapseChange: (id: string) => void;
  matchedSpanIds?: string[];
  parentPath?: PathEnum[];
  isFirstLevel?: boolean;
  isLastChild?: boolean;
}): TreeNode => {
  const lineStyle = spanStatusConfig[spanNode.status]?.lineStyle;
  const isMatched = matchedSpanIds?.includes(spanNode.span_id || '') ?? false;

  const treeNode: TreeNode = {
    key: spanNode.span_id || '',
    title: node => (
      <CustomTreeNode nodeData={node} onCollapseChange={onCollapseChange} />
    ),
    selectEnabled: ![BROKEN_ROOT_SPAN_ID, NORMAL_BROKEN_SPAN_ID].includes(
      spanNode.span_id,
    ),
    indentDisabled: spanNode.span_id === NORMAL_BROKEN_SPAN_ID,
    lineStyle,
    isLeaf: spanNode?.isLeaf,
    isLastChild,
    isMatched,
    linePath: isFirstLevel ? [] : [...parentPath, 1],
    zIndex: spanNode.status === SpanStatus.Error ? 1 : 0,
    extra: {
      spanNode,
    },
  };

  treeNode.children = spanNode.isCollapsed
    ? []
    : (spanNode.children?.map((childSpan, index) => {
        const isLast = index + 1 === spanNode.children?.length;
        const parent = isFirstLevel
          ? []
          : [...parentPath, isLastChild ? PathEnum.Hidden : PathEnum.Show];
        return spanNode2TreeNode({
          spanNode: childSpan,
          onCollapseChange,
          matchedSpanIds,
          isFirstLevel: false,
          isLastChild: isLast,
          parentPath: parent,
        });
      }) ?? []);

  return treeNode;
};
export const dealTreeNodeHighlight = (
  treeNode: TreeNode,
  selectedSpanId?: string,
) => {
  const selectedIdPath = findSelectedIdPath(treeNode, selectedSpanId || '');
  treeNode.children = transTreeNodeLinePath(
    treeNode.children || [],
    selectedIdPath,
    treeNode.linePath || [],
  );
  return treeNode;
};

const transTreeNodeLinePath = (
  treeNodes: TreeNode[],
  selectedIdPath: number[],
  parentPath: number[],
) =>
  treeNodes?.map((node, index) => {
    const currentPath = [...parentPath, index];
    const newLinePath = dealLinePath(
      node.linePath || [],
      selectedIdPath,
      currentPath,
    );
    node.linePath = newLinePath;
    if (node?.children && node?.children?.length) {
      node.children = transTreeNodeLinePath(
        node.children,
        selectedIdPath,
        currentPath,
      );
    }
    return {
      ...node,
    };
  });
const dealLinePath = (
  currentLinePath: PathEnum[],
  selectedIdPath: number[],
  currentIdPath: number[],
) => {
  let isFinish = false;
  return currentLinePath.map((line, index) => {
    const newLine = line === PathEnum.Active ? PathEnum.Show : line;
    const showActiveLine =
      line === PathEnum.Hidden ? PathEnum.Hidden : PathEnum.Active;
    if (isFinish) {
      return newLine;
    }
    if (selectedIdPath[index] > currentIdPath?.[index]) {
      isFinish = true;
      return showActiveLine;
    }
    if (
      index === currentIdPath.length - 1 &&
      selectedIdPath[index] === currentIdPath[index]
    ) {
      isFinish = true;
      return showActiveLine;
    }
    if (selectedIdPath[index] !== currentIdPath[index]) {
      isFinish = true;
    }
    return newLine;
  });
};

const findSelectedIdPath = (
  spanNode: TreeNode,
  selectedSpanId: string,
): number[] => {
  const traverse = (nodes: TreeNode[], path: number[]) => {
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      // 如果当前节点匹配，返回当前路径
      if (node.key === selectedSpanId) {
        return [...path, i];
      }
      // 如果有子节点，递归查找
      if (node.children && node.children.length > 0) {
        const result = traverse(node.children, [...path, i]);
        // 找到路径则直接返回
        if (result) {
          return result;
        }
      }
    }
    return null; // 未找到返回null
  };

  // 调用遍历函数，若未找到返回空数组
  return traverse(spanNode?.children || [], []) || [];
};
