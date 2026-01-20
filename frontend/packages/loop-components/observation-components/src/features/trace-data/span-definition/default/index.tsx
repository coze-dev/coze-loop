// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { z } from 'zod';

import { safeJsonParse } from '@/shared/utils/json';
import {
  type Span,
  type SpanDefinition,
  TagType,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { RawContent } from '../../components';

const parseReasoningContent = (outout: string) => {
  const jsonObj = safeJsonParse(outout);
  if (typeof jsonObj === 'string') {
    return '';
  }
  try {
    return (jsonObj as any)?.choices?.reduce(
      (pre, cur) => pre + (cur?.message?.reasoning_content ?? ''),
      '',
    ) as string;
  } catch (e) {
    return '';
  }
};

export const DEFAULT_SPAN_NAME = 'cozeloop-default-span-definition';

export class DefaultSpanDefinition
  implements SpanDefinition<string, string, string>
{
  name = DEFAULT_SPAN_NAME;
  inputSchema = z.any();
  outputSchema = z.any();
  parseSpanContent = (span: Span) => {
    const { input, output } = span;
    const { error } = span.custom_tags ?? {};
    const tools = (safeJsonParse(input) as any)?.tools;
    const reasonStr = parseReasoningContent(output);

    return {
      error: {
        isValidate: true,
        isEmpty: !error,
        content: error,
        originalContent: error,
        tagType: TagType.Error,
      },
      tool: {
        isValidate: true,
        isEmpty: !tools,
        content: tools,
        originalContent: tools,
        tagType: TagType.Functions,
      },
      input: {
        isValidate: true,
        isEmpty: !input,
        content: input,
        originalContent: input,
        tagType: TagType.Input,
      },
      reasoningContent: {
        isValidate: true,
        isEmpty: !reasonStr,
        content: reasonStr,
        originalContent: reasonStr,
        tagType: TagType.ReasoningContent,
      },
      output: {
        isValidate: true,
        isEmpty: !output,
        content: output,
        originalContent: output,
        tagType: TagType.Output,
      },
    };
  };

  renderError(
    span: Span,
    errorContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return (
      <RawContent
        structuredContent={errorContent}
        tagType={TagType.Error}
        attrTos={span.attr_tos}
        span={span}
      />
    );
  }
  renderTool(
    span: Span,
    toolContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return (
      <RawContent
        structuredContent={toolContent}
        tagType={TagType.Functions}
        attrTos={span.attr_tos}
        span={span}
      />
    );
  }
  renderInput(
    span: Span,
    inputContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return (
      <RawContent
        structuredContent={inputContent}
        tagType={TagType.Input}
        attrTos={span.attr_tos}
        span={span}
      />
    );
  }
  renderReasoningContent(
    span: Span,
    reasoningContent: string | undefined,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return (
      <RawContent
        structuredContent={reasoningContent ?? ''}
        tagType={TagType.ReasoningContent}
        attrTos={span.attr_tos}
        span={span}
      />
    );
  }
  renderOutput(
    span: Span,
    outputContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return (
      <RawContent
        structuredContent={outputContent}
        tagType={TagType.Output}
        attrTos={span.attr_tos}
        span={span}
      />
    );
  }
}
