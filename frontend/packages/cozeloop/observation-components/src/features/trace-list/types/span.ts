// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type {
  ListSpansResponse,
  TokenCost,
} from '@cozeloop/api-schema/observation';

export type Span = ListSpansResponse['spans'][number];

export type ConvertSpan = Span & {
  advanceInfoReady?: boolean;
  tokens?: TokenCost;
  spanType?: string | number;
};
