// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type span,
  type TraceAdvanceInfo,
} from '@cozeloop/api-schema/observation';

export interface DataSource {
  spans: span.OutputSpan[];
  advanceInfo?: TraceAdvanceInfo;
}

export const enum SpanType {
  Unknown = 'unknown',
  Model = 'model',
  Prompt = 'prompt',
  Parser = 'parser',
  Embedding = 'embedding',
  Memory = 'memory',
  Plugin = 'plugin',
  Function = 'function',
  Graph = 'graph',
  Remote = 'remote',
  Loader = 'loader',
  Transformer = 'transformer',
  VectorStore = 'vector_store',
  VectorRetriever = 'vector_retriever',
  Agent = 'agent',
  CozeBot = 'bot',
  LLMCall = 'llm_call',
}
