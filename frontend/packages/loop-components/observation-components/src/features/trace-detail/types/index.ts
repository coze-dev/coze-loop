// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type span } from '@cozeloop/api-schema/observation';

export interface JumpButtonConfig {
  visible?: boolean;
  onClick?: (span: span.OutputSpan) => void;
  text?: string;
}
