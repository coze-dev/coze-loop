// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import type React from 'react';

import { type ZodType } from 'zod';
import { type GetTraceResponse } from '@cozeloop/api-schema/observation';

export type Span = GetTraceResponse['spans'][0];
interface ParseSpanResult<Tool = unknown, Input = unknown, Output = unknown> {
  error: {
    isValidate: boolean;
    isEmpty: boolean;
    content?: string;
    originalContent?: string | object;
    tagType: TagType;
  };
  tool: {
    isValidate: boolean;
    isEmpty: boolean;
    content: Tool;
    originalContent?: string | object;
    tagType: TagType;
  };
  input: {
    isValidate: boolean;
    isEmpty: boolean;
    content: Input;
    originalContent?: string | object;
    tagType: TagType;
  };
  output: {
    isValidate: boolean;
    isEmpty: boolean;
    content: Output;
    originalContent?: string | object;
    tagType: TagType;
  };
  reasoningContent: {
    isValidate: boolean;
    isEmpty: boolean;
    content?: string;
    originalContent?: string | object;
    tagType: TagType;
  };
}

/**
 *  结构化 span 渲染插件定义
 */
export interface SpanDefinition<
  Tool = unknown,
  Input = unknown,
  Output = unknown,
> {
  /**
   * 结构化渲染 span input schema 定义
   */
  inputSchema: ZodType;
  /**
   * 结构化渲染 span output schema 定义
   */
  outputSchema: ZodType;

  /**
   * 结构化渲染 span name 定义 （每个定义应该有唯一的名字）
   */
  name: string;
  /**
   * @description 定义解析 span 的方法
   */
  parseSpanContent: (span: Span) => ParseSpanResult<Tool, Input, Output>;
  /**
   * @description 定义如何渲染错误
   */
  renderError: (span: Span, error: string) => React.ReactNode;
  /**
   * @description 定义如何渲染 Tool
   */
  renderTool: (span: Span, tool: Tool) => React.ReactNode;
  renderInput: (span: Span, input: Input) => React.ReactNode;
  renderReasoningContent: (
    span: Span,
    reasoningContent?: string,
  ) => React.ReactNode;
  renderOutput: (span: Span, output: Output) => React.ReactNode;
}

export type StructDataList = keyof ParseSpanResult;

export enum TagType {
  Input = 'input',
  Output = 'output',
  Error = 'error',
  Functions = 'functions',
  ReasoningContent = 'reasoning_content',
}

interface ToolCall {
  type: string;
  function: {
    name: string;
    arguments?: string;
  };
}

interface Part {
  type: string;
  text?: string;
  image_url?: {
    name?: string;
    url?: string;
    detail?: string;
  };
  file_url?: {
    name?: string;
    url?: string;
    detail?: string;
    suffix?: string;
  };
}

export interface RawMessage {
  role: string;
  content?: string | object | null;
  tool_calls?: ToolCall[];
  parts?: Part[];
  reasoning_content?: string;
}
