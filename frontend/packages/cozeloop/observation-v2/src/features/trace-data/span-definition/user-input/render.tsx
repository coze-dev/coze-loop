/* eslint-disable max-params */
import React from 'react';

import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  TagType,
  type RawMessage,
  type Span,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { SpanFieldRender } from '../../components/span-field-render';
import { RawContent } from '../../components/raw-content';
import type { UserInputSchema } from './schema';
import type { UserInputData } from './index';

interface UserInputDataRender {
  input: (
    input: RemoveUndefinedOrString<UserInputData>,
    attrTos: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  error: (
    error?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  output: (
    output: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
}

// 将 UserInput 数据转换为 RawMessage 格式
const convertUserInputToRawMessages = (
  userInput: UserInputSchema,
): RawMessage[] =>
  userInput.map(item => {
    const parts: RawMessage['parts'] = [];

    // 添加文本部分
    if (item.content.text) {
      parts.push({
        type: 'text',
        text: item.content.text,
      });
    }

    // 添加图片部分
    if (item.content.image_url) {
      parts.push({
        type: 'image_url',
        image_url: {
          url: item.content.image_url.url,
          name: item.content.image_url.name,
          detail: item.content.image_url.detail,
        },
      });
    }

    // 添加文件部分
    if (item.content.file_url) {
      parts.push({
        type: 'file_url',
        file_url: {
          url: item.content.file_url.url,
          name: item.content.file_url.file_name,
          suffix: item.content.file_url.suffix_type,
        },
      });
    }

    return {
      role: `user_input_${item.content_type}`,
      content: null,
      parts,
    };
  });

export const UserInputDataRender: UserInputDataRender = {
  input: (input, attrTos, spanRenderConfig, span) => {
    if (typeof input === 'string' || !Array.isArray(input)) {
      return (
        <RawContent
          structuredContent={input as string}
          tagType={TagType.Input}
          span={span}
        />
      );
    }

    const rawMessages = convertUserInputToRawMessages(input);

    return (
      <SpanFieldRender
        attrTos={attrTos}
        messages={rawMessages}
        tagType={TagType.Input}
        spanRenderConfig={spanRenderConfig}
        span={span}
      />
    );
  },
  error: (error?: string, spanRenderConfig?: SpanRenderConfig, span?: Span) => (
    <RawContent
      structuredContent={error ?? ''}
      tagType={TagType.Error}
      span={span}
    />
  ),

  output: (output, spanRenderConfig, span) => (
    <RawContent
      structuredContent={output ?? ''}
      tagType={TagType.Output}
      span={span}
    />
  ),
};
