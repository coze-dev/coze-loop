// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isEmpty } from 'lodash-es';

import { safeJsonParse } from '../../utils/json';
import { type RemoveUndefinedOrString } from '../../types/utils';
import { TagType, type Span, type SpanDefinition } from '../../types';
import {
  modelInputSchema,
  modelOutputSchema,
  type ModelInputSchema,
  type ModelOutputSchema,
} from './schema';
import { ModelDataRender } from './render';

export type Tool = ModelInputSchema['tools'] | string | undefined;
export type Input = ModelInputSchema['messages'] | string;
export type Output = ModelOutputSchema | string;

const getInputAndTools = (input: string) => {
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

  const { tools, messages } = validateInput.data;

  const inputContent = {
    isValidate: true,
    isEmpty: isEmpty(messages),
    content: messages.map(m => ({
      role: m.role,
      reasoningContent: m.reasoning_content,
      parts: m.parts,
      content: m.content,
      tool_calls: m.tool_calls,
    })),
    originalContent: input,
    tagType: TagType.Input,
  };

  const toolContent = {
    isValidate: true,
    isEmpty: isEmpty(tools),
    content: tools,
    originalContent: tools,
    tagType: TagType.Functions,
  };

  return {
    input: inputContent,
    tool: toolContent,
  };
};

const getOutputAndReasoningContent = (output: string) => {
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

  const { choices } = validateOutput.data;

  const reasoningStr = choices.reduce(
    (pre, cur) => pre + (cur.message.reasoning_content ?? ''),
    '',
  );
  const reasoningContent = {
    content: reasoningStr,
    isEmpty: !reasoningStr,
    isValidate: true,
    originalContent: output,
    tagType: TagType.ReasoningContent,
  };

  const outputContent = {
    choices: choices.map(c => ({
      message: {
        tool_calls: c.message.tool_calls,
        role: c.message.role,
        content: c.message.content,
        reasoning_content: c.message.reasoning_content,
        parts: c.message.parts,
      },
    })),
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
  implements SpanDefinition<Tool, Input, Output>
{
  name = 'model';
  inputSchema = modelInputSchema;
  outputSchema = modelOutputSchema;
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
    } as const;
  };

  renderError(_span: Span, errorContent: string) {
    return ModelDataRender.error(errorContent);
  }
  renderInput(_span: Span, inputContent: Input) {
    return ModelDataRender.input(
      inputContent as RemoveUndefinedOrString<Input>,
      _span.attr_tos,
    );
  }

  renderOutput(_span: Span, outputContent: Output) {
    return ModelDataRender.output(
      outputContent as RemoveUndefinedOrString<Output>,
      _span.attr_tos,
    );
  }
  renderTool(_span: Span, toolContent: Tool) {
    return ModelDataRender.tool(toolContent as RemoveUndefinedOrString<Tool>);
  }
  renderReasoningContent(_span: Span, reasoningContent: string | undefined) {
    return ModelDataRender.reasoningContent(reasoningContent);
  }
}
