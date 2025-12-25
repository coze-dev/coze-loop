// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';

import { safeJsonParse } from '@/shared/utils/json';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  TagType,
  type Span,
  type SpanDefinition,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import {
  retrieverInputSchema,
  retrieverOutputSchema,
  type RetrieverInputSchema,
  type RetrieverOutputSchema,
} from './schema';
import { RetrieverDataRender } from './render';

export type RetrieverInputData = RetrieverInputSchema | string;
export type RetrieverOutputData = RetrieverOutputSchema | string;

const getRetrieverInputContent = (input: string) => {
  const parsedInput = safeJsonParse(input);

  const validateInput = retrieverInputSchema.safeParse(parsedInput);

  if (typeof parsedInput === 'string' || !validateInput.success) {
    return {
      content: input,
      isValidate: false,
      isEmpty: !input,
      originalContent: input,
    };
  }

  const inputData = validateInput.data;

  return {
    isValidate: true,
    isEmpty: !inputData.query,
    content: inputData,
    originalContent: input,
  };
};

const getRetrieverOutputContent = (output: string) => {
  const parsedOutput = safeJsonParse(output);

  const validateOutput = retrieverOutputSchema.safeParse(parsedOutput);

  if (typeof parsedOutput === 'string' || !validateOutput.success) {
    return {
      content: output,
      isValidate: false,
      isEmpty: !output,
      originalContent: output,
    };
  }

  const outputData = validateOutput.data;

  return {
    isValidate: true,
    isEmpty: isEmpty(outputData.documents),
    content: outputData,
    originalContent: output,
  };
};

export class RetrieverSpanDefinition
  implements SpanDefinition<undefined, RetrieverInputData, RetrieverOutputData>
{
  name = 'retriever';
  inputSchema = retrieverInputSchema;
  outputSchema = retrieverOutputSchema;
  parseSpanContent = (span: Span) => {
    const { input, output } = span;
    const { error } = span.custom_tags ?? {};

    const inputContent = getRetrieverInputContent(input);
    const outputContent = getRetrieverOutputContent(output);

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
        isEmpty: true,
        content: undefined,
        originalContent: undefined,
        tagType: TagType.Functions,
      },
      input: {
        ...inputContent,
        tagType: TagType.Input,
      },
      output: {
        ...outputContent,
        tagType: TagType.Output,
      },
      reasoningContent: {
        isValidate: true,
        isEmpty: true,
        content: undefined,
        originalContent: undefined,
        tagType: TagType.ReasoningContent,
      },
    };
  };

  renderError(
    span: Span,
    errorContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return RetrieverDataRender.error(errorContent, spanRenderConfig, span);
  }

  renderInput(
    span: Span,
    inputContent: RetrieverInputData,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return RetrieverDataRender.input(
      inputContent as RemoveUndefinedOrString<RetrieverInputData>,
      spanRenderConfig,
      span,
    );
  }

  renderOutput(
    span: Span,
    outputContent: RetrieverOutputData,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return RetrieverDataRender.output(
      outputContent as RemoveUndefinedOrString<RetrieverOutputData>,
      spanRenderConfig,
      span,
    );
  }

  renderTool(
    _span: Span,
    toolContent: undefined,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return null;
  }

  renderReasoningContent(
    _span: Span,
    reasoningContent: string | undefined,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return null;
  }
}
