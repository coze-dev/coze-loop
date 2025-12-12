export const QUERY_PROPERTY = {
  Status: 'status',
  TraceId: 'trace_id',
  Input: 'input',
  Output: 'output',
  Tokens: 'tokens',
  InputTokens: 'input_tokens',
  OutputTokens: 'output_tokens',
  Latency: 'latency',
  LatencyFirst: 'latency_first_resp',
  SpanId: 'span_id',
  SpanName: 'span_name',
  SpanType: 'span_type',
  PromptKey: 'prompt_key',
  LogicDeleteDate: 'logic_delete_date',
  StartTime: 'start_time',
  Feedback: 'feedback',
};

export type QueryPropertyEnum =
  (typeof QUERY_PROPERTY)[keyof typeof QUERY_PROPERTY];
