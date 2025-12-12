/* eslint-disable max-params */
import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';

/**
 * 过滤非关键节点的函数
 * @param nodes 节点数组
 * @param filterNonCritical 是否启用过滤
 * @param level 当前层级，默认为1
 * @param keySpanType 关键节点类型列表
 * @returns 过滤后的节点数组
 */
export const filterCriticalNodes = (
  nodes: SpanNode[],
  filterNonCritical: boolean,
  level = 1,
  keySpanType?: string[],
): SpanNode[] => {
  if (!filterNonCritical) {
    return nodes;
  }

  const result: SpanNode[] = [];

  for (const node of nodes) {
    // 第一层节点不过滤，直接保留
    if (level === 1) {
      result.push({
        ...node,
        children: node.children
          ? filterCriticalNodes(
              node.children as SpanNode[],
              filterNonCritical,
              level + 1,
              keySpanType,
            )
          : undefined,
      });
      continue;
    }

    // 第二层及以下节点需要过滤
    const shouldKeepNode = keySpanType?.includes(node.span_type) ?? false;

    if (shouldKeepNode) {
      // 如果当前节点保留，递归处理子节点
      result.push({
        ...node,
        children: node.children
          ? filterCriticalNodes(
              node.children as SpanNode[],
              filterNonCritical,
              level + 1,
              keySpanType,
            )
          : undefined,
      });
    } else {
      // 如果当前节点被过滤，但有子节点，则将子节点提升到当前层级
      if (node.children && node.children.length > 0) {
        result.push(
          ...filterCriticalNodes(
            node.children as SpanNode[],
            filterNonCritical,
            level,
            keySpanType,
          ),
        );
      }
      // 如果节点被过滤且没有子节点，则不添加到结果中
    }
  }

  return result;
};
