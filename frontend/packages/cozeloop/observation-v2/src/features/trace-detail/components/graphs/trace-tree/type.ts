import { type OutputSpan } from '@cozeloop/api-schema/observation';

import { type TreeProps } from '../tree/typing';

export type TraceTreeProps = {
  dataSource: SpanNode[];
  selectedSpanId?: string;
  matchedSpanIds?: string[];
  onCollapseChange: (id: string) => void;
} & Pick<
  TreeProps,
  | 'indentDisabled'
  | 'lineStyle'
  | 'globalStyle'
  | 'onSelect'
  | 'onClick'
  | 'onMouseMove'
  | 'onMouseEnter'
  | 'onMouseLeave'
  | 'className'
  | 'virtuosoHeight'
>;

export type SpanNode = OutputSpan & {
  children?: SpanNode[];
  isCollapsed: boolean;
  isLeaf: boolean;
};
