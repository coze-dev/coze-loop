// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { z } from 'zod';
import { isEmpty } from 'lodash-es';

import { safeJsonParse } from '@/shared/utils/json';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  TagType,
  type Span,
  type SpanDefinition,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { userInputSchema, type UserInputSchema } from './schema';
import { UserInputDataRender } from './render';

export type UserInputData = UserInputSchema | string;

const getUserInputContent = (input: string) => {
  const parsedInput = safeJsonParse(input);

  const validateInput = userInputSchema.safeParse(parsedInput);

  if (typeof parsedInput === 'string' || !validateInput.success) {
    return {
      input: {
        content: input,
        isValidate: false,
        isEmpty: !input,
        originalContent: input,
        tagType: TagType.Input,
      },
    };
  }

  const userInputData = validateInput.data;

  const inputContent = {
    isValidate: true,
    isEmpty: isEmpty(userInputData),
    content: userInputData,
    originalContent: input,
    tagType: TagType.Input,
  };

  return {
    input: inputContent,
  };
};

export class UserInputSpanDefinition
  implements SpanDefinition<undefined, UserInputData, string>
{
  name = 'UserInput';
  inputSchema = userInputSchema;
  outputSchema = z.any();
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
      tool: {
        isValidate: true,
        isEmpty: true,
        content: undefined,
        originalContent: undefined,
        tagType: TagType.Functions,
      },
      output: {
        isValidate: true,
        isEmpty: !output,
        content: output,
        originalContent: output,
        tagType: TagType.Output,
      },
      reasoningContent: {
        isValidate: true,
        isEmpty: true,
        content: undefined,
        originalContent: undefined,
        tagType: TagType.ReasoningContent,
      },
      ...getUserInputContent(input),
    };
  };

  renderError(
    _span: Span,
    errorContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return UserInputDataRender.error(errorContent, spanRenderConfig);
  }

  renderInput(
    _span: Span,
    inputContent: UserInputData,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return UserInputDataRender.input(
      inputContent as RemoveUndefinedOrString<UserInputData>,
      _span.attr_tos,
      spanRenderConfig,
    );
  }

  renderOutput(
    _span: Span,
    outputContent: string,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return UserInputDataRender.output(outputContent, spanRenderConfig);
  }

  renderTool(
    _span: Span,
    _toolContent: undefined,
    _spanRenderConfig?: SpanRenderConfig,
  ) {
    return null;
  }

  renderReasoningContent(
    _span: Span,
    _reasoningContent: string | undefined,
    _spanRenderConfig?: SpanRenderConfig,
  ) {
    return null;
  }
}
