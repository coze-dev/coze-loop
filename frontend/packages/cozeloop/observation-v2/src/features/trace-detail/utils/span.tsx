import { keyBy, uniqBy } from 'lodash-es';
import {
  type OutputSpan,
  SpanStatus,
  SpanType,
} from '@cozeloop/api-schema/observation';

import {
  BROKEN_ROOT_SPAN_ID,
  NODE_CONFIG_MAP,
  NORMAL_BROKEN_SPAN_ID,
  type NodeConfig,
} from '@/features/trace-detail/constants/span';
import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';
/** 数组转换成链式节点 */
export function spans2SpanNodes(spans: OutputSpan[]) {
  if (spans.length === 0) {
    return;
  }

  const roots: SpanNode[] = [];
  const map: Record<string, SpanNode> = {};

  // 排序 + 去重
  const sortedSpans = uniqBy(spans, span => span.span_id).sort((a, b) => {
    const startA = a.started_at ? Number(a.started_at) : Infinity;
    const startB = b.started_at ? Number(b.started_at) : Infinity;
    return startA - startB;
  });

  sortedSpans.forEach(span => {
    const currentSpan: SpanNode = {
      ...span,
      children: [],
      isLeaf: true,
      isCollapsed: false,
    };
    const { span_id } = span;
    if (span_id) {
      map[span_id] = currentSpan;
    }
  });

  sortedSpans.forEach(span => {
    const { span_id, parent_id } = span;
    if (span_id) {
      const spanNode = map[span_id];
      const parentSpanNode = parent_id ? map[parent_id] : undefined;
      if (parent_id === '0' || parentSpanNode === undefined) {
        roots.push(spanNode);
      } else {
        parentSpanNode.children = parentSpanNode.children ?? [];
        parentSpanNode.children.push(spanNode);
        parentSpanNode.isLeaf = false;
      }
    }
  });

  // const trueRoot = roots.find(root => root.parent_id === '0');
  // const brokenNodes = roots.filter(root => root.parent_id !== '0');

  // if (!trueRoot) {
  //   return appendBrokenToBrokenRoot(brokenNodes);
  // } else if (brokenNodes.length > 0) {
  //   return appendBrokenNodesToRoot(trueRoot, brokenNodes);
  // }
  // return trueRoot;
  return roots;
}

/** 把没有父节点的非根节点挂在到虚拟根节点上 */
export function appendBrokenToBrokenRoot(brokenNodes: SpanNode[]) {
  const vRoot: SpanNode = {
    span_id: BROKEN_ROOT_SPAN_ID,
    parent_id: '0',
    trace_id: '',
    span_name: '',
    type: SpanType.Unknown,
    status: SpanStatus.Success,
    span_type: '',
    status_code: 0,
    started_at: '',
    duration: '',
    input: '',
    output: '',
    custom_tags: {
      device_id: '',
      space_id: '',
      psm_env: '',
      err_msg: '',
      user_id: '',
      psm: '',
    },
    isCollapsed: false,
    isLeaf: false,
    children: brokenNodes,
  };
  return vRoot;
}

/** 把没有父节点的非根节点挂载root上 */
export function appendBrokenNodesToRoot(
  rootNode: SpanNode,
  brokenNodes: SpanNode[],
) {
  const brokenRoot: SpanNode = {
    span_id: NORMAL_BROKEN_SPAN_ID,
    parent_id: rootNode.span_id,
    trace_id: '',
    span_name: '',
    type: SpanType.Unknown,
    status: SpanStatus.Success,
    started_at: '',
    duration: '',
    span_type: '',
    status_code: 0,
    input: '',
    output: '',
    custom_tags: {
      device_id: '',
      space_id: '',
      psm_env: '',
      err_msg: '',
      user_id: '',
      psm: '',
    },
    isCollapsed: false,
    isLeaf: false,
    children: brokenNodes,
  };

  rootNode.children?.push(brokenRoot);
  return rootNode;
}

interface GetNodeConfigParameters {
  /** span type 枚举映射 */
  spanTypeEnum: SpanNode['type'];
  /** span type 字符串，用户真实上报的字段 */
  spanType: string;
}

export function getNodeConfig(params: GetNodeConfigParameters): NodeConfig {
  const { spanTypeEnum, spanType } = params;

  let nodeConfig = NODE_CONFIG_MAP[spanTypeEnum];
  if (!nodeConfig) {
    nodeConfig = NODE_CONFIG_MAP.unknown;
  }

  return {
    ...nodeConfig,
    typeName: spanType ?? '-',
  };
}

export function changeSpanNodeCollapseStatus(
  spanNodes: SpanNode[],
  targetId: string,
): SpanNode[] {
  return spanNodes.map(node => {
    if (node.span_id === targetId) {
      return {
        ...node,
        isCollapsed: !node.isCollapsed,
      };
    }

    if (node.children) {
      return {
        ...node,
        children: changeSpanNodeCollapseStatus(node.children, targetId),
      };
    } else {
      return node;
    }
  });
}

/**
 * 一键展开所有节点
 */
export function expandAllSpanNodes(spanNodes: SpanNode[]): SpanNode[] {
  return spanNodes.map(node => ({
    ...node,
    isCollapsed: false,
    children: node.children ? expandAllSpanNodes(node.children) : undefined,
  }));
}

// DFS 查找从根到目标节点的祖先路径（不包含目标本身）
export const findPathToSpan = (
  targetId: string,
  rootsList: SpanNode[] = [],
): SpanNode[] => {
  const dfs = (node: SpanNode, acc: SpanNode[]): boolean => {
    if (!node) {
      return false;
    }
    if (node.span_id === targetId) {
      return true;
    }
    if (node.children && node.children.length) {
      acc.push(node);
      for (const child of node.children) {
        if (dfs(child as SpanNode, acc)) {
          return true;
        }
      }
      acc.pop();
    }
    return false;
  };
  for (const root of rootsList) {
    const path: SpanNode[] = [];
    const found = dfs(root, path);
    if (found) {
      return path;
    }
  }
  return [];
};
/**
 * 一键收起所有节点
 * 注意：最顶层的根节点不收起，只收起子节点
 */
export function collapseAllSpanNodes(spanNodes: SpanNode[]): SpanNode[] {
  return spanNodes.map(node => ({
    ...node,
    // 根节点保持展开状态，只收起其子节点
    isCollapsed: false,
    children: node.children ? collapseAllChildNodes(node.children) : undefined,
  }));
}

/**
 * 收起所有子节点（递归收起所有层级）
 */
function collapseAllChildNodes(spanNodes: SpanNode[]): SpanNode[] {
  return spanNodes.map(node => ({
    ...node,
    isCollapsed: true,
    children: node.children ? collapseAllChildNodes(node.children) : undefined,
  }));
}

/**
 * 切换所有节点的展开收起状态
 */
export function toggleAllSpanNodes(
  spanNodes: SpanNode[],
  expand: boolean,
): SpanNode[] {
  return expand
    ? expandAllSpanNodes(spanNodes)
    : collapseAllSpanNodes(spanNodes);
}

/**
 * 检查是否所有节点都已展开
 * 注意：只检查子节点的展开状态，根节点始终被视为展开状态
 */
export function checkAllNodesExpanded(spanNodes: SpanNode[]): boolean {
  return spanNodes.every(node => {
    if (node.isLeaf) {
      return true; // 叶子节点不需要展开
    }
    // 根节点不检查其自身的展开状态，只检查其子节点
    return node.children ? checkAllChildNodesExpanded(node.children) : true;
  });
}

/**
 * 检查所有子节点是否都已展开（递归检查所有层级）
 */
function checkAllChildNodesExpanded(spanNodes: SpanNode[]): boolean {
  return spanNodes.every(node => {
    if (node.isLeaf) {
      return true; // 叶子节点不需要展开
    }
    if (node.isCollapsed) {
      return false; // 有节点是收起的
    }
    // 递归检查子节点
    return node.children ? checkAllChildNodesExpanded(node.children) : true;
  });
}

// eslint-disable-next-line  @typescript-eslint/no-explicit-any
type AnyObject = Record<string, any>;
export interface RowMessage {
  role: string;
  content: string | object;
  tool_calls?: AnyObject[];
  parts?: AnyObject[];
  reasoningContent?: string;
}

export enum SpanContentType {
  Model = 'model',
  Prompt = 'prompt',
}

export const getRootSpan = (spans: OutputSpan[]) => {
  if (spans.length === 0) {
    return;
  }

  // 排序 + 去重
  const sortedSpans = uniqBy(spans, span => span.span_id).sort((a, b) => {
    const startA = a.started_at ? Number(a.started_at) : Infinity;
    const startB = b.started_at ? Number(b.started_at) : Infinity;
    return startA - startB;
  });

  const map = keyBy(sortedSpans, 'span_id');

  for (const span of sortedSpans) {
    const { span_id, parent_id } = span;
    if (span_id) {
      const parentSpan = parent_id ? map[parent_id] : undefined;
      if (parent_id === '0' || parentSpan === undefined) {
        return span;
      }
    }
  }
};

//获取根节点的数量（包含子节点）
export const getRootNodesLengthWithChildren = (rootNodes?: SpanNode[]) => {
  let count = 0;
  for (const root of rootNodes || []) {
    count += 1;
    if (root.children) {
      count += getRootNodesLengthWithChildren(root.children);
    }
  }
  return count;
};

// 判断rootnodes中的节点层级是否小于2层
export const isRootNodesDepthLessThan2 = (rootNodes?: SpanNode[]) => {
  if (!rootNodes || rootNodes.length === 0) {
    return true;
  }

  // 递归计算节点的最大深度
  const getMaxDepth = (nodes: SpanNode[], currentDepth = 0): number => {
    let maxDepth = currentDepth;

    for (const node of nodes) {
      if (node.children && node.children.length > 0) {
        const childDepth = getMaxDepth(node.children, currentDepth + 1);
        maxDepth = Math.max(maxDepth, childDepth);
      }
    }

    return maxDepth;
  };

  const maxDepth = getMaxDepth(rootNodes);
  return maxDepth < 2;
};
