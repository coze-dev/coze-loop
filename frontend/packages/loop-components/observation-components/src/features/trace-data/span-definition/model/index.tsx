// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';
import { type span } from '@cozeloop/api-schema/observation';

import { safeJsonParse } from '@/shared/utils/json';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  TagType,
  type RawMessage,
  type Span,
  type SpanDefinition,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import {
  normalizeTools,
  normalizeInputMessages,
  normalizeOutputChoices,
} from './utils';
import {
  modelInputSchema,
  modelOutputSchema,
  type ModelInputSchema,
  type ModelOutputSchema,
} from './schema';
import { ModelDataRender } from './render';

type ModelOutputWithChoices = Extract<ModelOutputSchema, { choices: unknown }>;

export type Tool = ModelInputSchema['tools'] | string | undefined;
export type Input = RawMessage[] | string;
export type Output = ModelOutputWithChoices | string;

export const getInputAndTools = (input: string) => {
  const parsedInput = safeJsonParse(input);

  const validateInput = modelInputSchema.safeParse(parsedInput);

  if (typeof parsedInput === 'string' || !validateInput.success) {
    return {
      input: {
        content: input,
        isValidate: false,
        isEmpty: !input,
        originalContent: input,
        tagType: TagType.Input,
      },
      tool: {
        content: '',
        isValidate: false,
        isEmpty: true,
        originalContent: '',
        tagType: TagType.Functions,
      },
    };
  }

  const { tools, previous_response_id } = validateInput.data;

  const normalizedTools = normalizeTools(tools);

  const normalizedMessages = normalizeInputMessages(
    validateInput.data,
    previous_response_id,
  );

  const inputContent = {
    isValidate: true,
    isEmpty: isEmpty(normalizedMessages),
    content: normalizedMessages,
    originalContent: input,
    tagType: TagType.Input,
  };

  const toolContent = {
    isValidate: true,
    isEmpty: isEmpty(normalizedTools),
    content: normalizedTools,
    originalContent: tools ?? '',
    tagType: TagType.Functions,
  };

  return {
    input: inputContent,
    tool: toolContent,
  };
};

export const getOutputAndReasoningContent = (output: string) => {
  const parsedOutput = safeJsonParse(output);
  const validateOutput = modelOutputSchema.safeParse(parsedOutput);

  if (typeof parsedOutput === 'string' || !validateOutput.success) {
    return {
      output: {
        content: output,
        isValidate: false,
        isEmpty: !output,
        originalContent: output,

        tagType: TagType.Output,
      },
      reasoningContent: {
        content: '',
        isValidate: false,
        isEmpty: true,
        originalContent: '',
        tagType: TagType.ReasoningContent,
      },
    };
  }

  const choices = normalizeOutputChoices(validateOutput.data);

  const reasoningStr = choices.reduce(
    (pre, cur) => pre + (cur.message.reasoning_content ?? ''),
    '',
  );
  const reasoningContent = {
    content: reasoningStr,
    isEmpty: !reasoningStr,
    isValidate: true,
    originalContent: reasoningStr,
    tagType: TagType.ReasoningContent,
  };

  const outputContent: ModelOutputWithChoices = {
    choices,
  };

  return {
    reasoningContent,
    output: {
      content: outputContent,
      isEmpty: isEmpty(choices),
      isValidate: true,
      originalContent: output,
      tagType: TagType.Output,
    },
  };
};

export class ModelSpanDefinition
  implements SpanDefinition<Tool, Input, Output, span.OutputSpan[]>
{
  name = 'model';
  inputSchema = modelInputSchema;
  outputSchema = modelOutputSchema;
  context?: span.OutputSpan[] | undefined = [];
  setContext = (context: span.OutputSpan[]) => {
    this.context = context;
  };
  parseSpanContent = (span: Span) => {
    const { input, output } = span;
    const { error } = span.custom_tags ?? {};

    return {
      error: {
        isValidate: true,
        isEmpty: !error,
        content: error,
        originalContent: error,
        tagType: TagType.Error,
      },
      ...getInputAndTools(input),
      ...getOutputAndReasoningContent(output),
    };
  };

  renderError(
    span: Span,
    errorContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return ModelDataRender.error(errorContent, spanRenderConfig, span);
  }

  renderInput = (
    span: Span,
    inputContent: Input,
    spanRenderConfig?: SpanRenderConfig,
  ) =>
    ModelDataRender.input(
      inputContent as RemoveUndefinedOrString<Input>,
      span.attr_tos,
      spanRenderConfig,
      span,
      this.context,
    );

  renderOutput = (
    span: Span,
    outputContent: Output,
    spanRenderConfig?: SpanRenderConfig,
  ) =>
    ModelDataRender.output(
      outputContent as RemoveUndefinedOrString<Output>,
      span.attr_tos,
      spanRenderConfig,
      span,
      this.context,
    );
  renderTool(
    span: Span,
    toolContent: Tool,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return ModelDataRender.tool(
      toolContent as RemoveUndefinedOrString<Tool>,
      spanRenderConfig,
      span,
    );
  }
  renderReasoningContent(
    span: Span,
    reasoningContent: string | undefined,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return ModelDataRender.reasoningContent(
      reasoningContent,
      spanRenderConfig,
      span,
    );
  }
}
