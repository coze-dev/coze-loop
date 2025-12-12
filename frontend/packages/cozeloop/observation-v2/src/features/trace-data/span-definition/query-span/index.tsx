import { isEmpty } from 'lodash-es';

import { safeJsonParse } from '@/shared/utils/json';
import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  TagType,
  type Span,
  type SpanDefinition,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { querySpanSchema, type QuerySpanSchema } from './schema';
import { QuerySpanDataRender } from './render';

export type QuerySpanData = QuerySpanSchema | string;

const getQuerySpanContent = (data: string) => {
  const parsedData = safeJsonParse(data);

  const validateData = querySpanSchema.safeParse(parsedData);

  if (typeof parsedData === 'string' || !validateData.success) {
    return {
      content: data,
      isValidate: false,
      isEmpty: !data,
      originalContent: data,
      tagType: TagType.Input,
    };
  }

  const querySpanData = validateData.data;

  return {
    isValidate: true,
    isEmpty: isEmpty(querySpanData.contents),
    content: querySpanData,
    originalContent: data,
    tagType: TagType.Input,
  };
};

export class QuerySpanDefinition
  implements SpanDefinition<undefined, QuerySpanData, QuerySpanData>
{
  name = 'fornax_query';
  inputSchema = querySpanSchema;
  outputSchema = querySpanSchema;
  parseSpanContent = (span: Span) => {
    const { input, output } = span;
    const { error } = span.custom_tags ?? {};

    const inputContent = getQuerySpanContent(input);
    const outputContent = getQuerySpanContent(output);

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
    return QuerySpanDataRender.error(errorContent, spanRenderConfig, span);
  }

  renderInput(
    span: Span,
    inputContent: QuerySpanData,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return QuerySpanDataRender.input(
      inputContent as RemoveUndefinedOrString<QuerySpanData>,
      span.attr_tos,
      spanRenderConfig,
      span,
    );
  }

  renderOutput(
    span: Span,
    outputContent: QuerySpanData,
    spanRenderConfig?: SpanRenderConfig,
  ) {
    return QuerySpanDataRender.output(
      outputContent as RemoveUndefinedOrString<QuerySpanData>,
      span.attr_tos,
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
