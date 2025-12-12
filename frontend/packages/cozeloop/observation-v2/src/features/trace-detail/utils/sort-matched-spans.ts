import { type OutputSpan } from '@cozeloop/api-schema/observation';

import { type SpanNode } from '@/features/trace-detail/components/graphs/trace-tree/type';

// 对matchedSpans进行排序，按照rootNodes的树结构中出现的位置排序，深度优先，即0-1-1在0-2-0的前边
export const sortMatchedSpans = (
  matchedSpans: OutputSpan[],
  rootNodes?: SpanNode[],
) => {
  const matchedSpanIds = matchedSpans
    .map(r => r?.span_id)
    .filter(Boolean) as string[];
  const sortedSpans: string[] = [];

  const dfs = (node: SpanNode) => {
    if (matchedSpanIds.includes(node?.span_id)) {
      sortedSpans.push(node?.span_id);
    }
    node.children?.forEach(child => dfs(child));
  };
  rootNodes?.forEach(root => dfs(root));
  return sortedSpans;
};
