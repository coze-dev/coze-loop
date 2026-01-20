// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as annotation from './annotation';
export { annotation };
export enum SpanStatus {
  Success = "success",
  Error = "error",
  Broken = "broken",
}
export enum SpanType {
  Unknown = "unknwon",
  Prompt = "prompt",
  Model = "model",
  Parser = "parser",
  Embedding = "embedding",
  Memory = "memory",
  Plugin = "plugin",
  Function = "function",
  Graph = "graph",
  Remote = "remote",
  Loader = "loader",
  Transformer = "transformer",
  VectorStore = "vector_store",
  VectorRetriever = "vector_retriever",
  Agent = "agent",
  LLMCall = "LLMCall",
}
export interface AttrTos {
  input_data_url?: string,
  output_data_url?: string,
  multimodal_data?: {
    [key: string | number]: string
  },
}
export interface OutputSpan {
  trace_id: string,
  span_id: string,
  parent_id: string,
  span_name: string,
  span_type: string,
  type: SpanType,
  started_at: string,
  duration: string,
  status: SpanStatus,
  status_code: number,
  input: string,
  output: string,
  logic_delete_date?: string,
  service_name?: string,
  logid?: string,
  system_tags_string?: {
    [key: string | number]: string
  },
  system_tags_long?: {
    [key: string | number]: string
  },
  system_tags_double?: {
    [key: string | number]: number
  },
  tags_string?: {
    [key: string | number]: string
  },
  tags_long?: {
    [key: string | number]: string
  },
  tags_double?: {
    [key: string | number]: number
  },
  tags_bool?: {
    [key: string | number]: boolean
  },
  tags_bytes?: {
    [key: string | number]: string
  },
  call_type?: string,
  custom_tags?: {
    [key: string | number]: string
  },
  attr_tos?: AttrTos,
  system_tags?: {
    [key: string | number]: string
  },
  annotations?: annotation.Annotation[],
}
export interface InputSpan {
  started_at_micros: string,
  log_id?: string,
  span_id: string,
  parent_id: string,
  trace_id: string,
  duration: string,
  service_name?: string,
  call_type?: string,
  workspace_id: string,
  span_name: string,
  span_type: string,
  method: string,
  status_code: number,
  input: string,
  output: string,
  object_storage?: string,
  system_tags_string?: {
    [key: string | number]: string
  },
  system_tags_long?: {
    [key: string | number]: string
  },
  system_tags_double?: {
    [key: string | number]: number
  },
  tags_string?: {
    [key: string | number]: string
  },
  tags_long?: {
    [key: string | number]: string
  },
  tags_double?: {
    [key: string | number]: number
  },
  tags_bool?: {
    [key: string | number]: boolean
  },
  tags_bytes?: {
    [key: string | number]: string
  },
  duration_micros?: string,
}