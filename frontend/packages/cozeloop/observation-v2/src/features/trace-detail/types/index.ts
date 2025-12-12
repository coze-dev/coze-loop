import { type span } from '@cozeloop/api-schema/observation';

export interface JumpButtonConfig {
  visible?: boolean;
  onClick?: (span: span.OutputSpan) => void;
  text?: string;
}
