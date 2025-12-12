import {
  type TraceAdvanceInfo,
  type span,
} from '@cozeloop/api-schema/observation';

import {
  type BaseComponentProps,
  type ClosableProps,
} from '@/shared/types/utils';

import { type SwitchConfig } from '../../containers/trace-detail/interface';

export interface TraceHeaderProps extends BaseComponentProps, ClosableProps {
  rootSpan?: span.OutputSpan;
  switchConfig?: SwitchConfig;
  extraRender?: (span?: span.OutputSpan) => React.ReactNode;
  advanceInfo?: TraceAdvanceInfo;
  /** 自定义渲染头部复制节点 */
  renderHeaderCopyNode?: (span: span.OutputSpan) => React.ReactNode;
}
