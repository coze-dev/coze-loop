// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
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
import type { QuerySpanSchema } from './schema';
import type { QuerySpanData } from './index';

interface QuerySpanDataRender {
  input: (
    input: RemoveUndefinedOrString<QuerySpanData>,
    attrTos: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  output: (
    output: RemoveUndefinedOrString<QuerySpanData>,
    attrTos: Span['attr_tos'],
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
  error: (
    error?: string,
    spanRenderConfig?: SpanRenderConfig,
    span?: Span,
  ) => React.ReactNode;
}

// 将 QuerySpan 数据转换为 RawMessage 格式
const convertQuerySpanToRawMessages = (
  querySpan: QuerySpanSchema,
): RawMessage[] => {
  const parts: RawMessage['parts'] = [];

  querySpan.contents.forEach(item => {
    // 添加文本部分
    if (item.content_type === 'text' && item.text) {
      parts.push({
        type: 'text',
        text: item.text,
      });
    }

    // 添加图片部分
    if (item.content_type === 'image' && item.image) {
      parts.push({
        type: 'image_url',
        image_url: {
          url: item.image.url,
          name: item.image.name,
        },
      });
    }

    // 添加文件部分
    if (item.content_type === 'file' && item.file) {
      parts.push({
        type: 'file_url',
        file_url: {
          url: item.file.url,
          name: item.file.name,
          suffix: item.file.suffix,
        },
      });
    }
  });

  return [
    {
      role: 'query',
      content: null,
      parts,
    },
  ];
};

const renderQuerySpanContent = (
  data: RemoveUndefinedOrString<QuerySpanData>,
  attrTos: Span['attr_tos'],
  tagType: TagType,
  spanRenderConfig?: SpanRenderConfig,
  span?: Span,
) => {
  if (typeof data === 'string' || !data || !('contents' in data)) {
    return (
      <RawContent
        structuredContent={data as string}
        tagType={tagType}
        span={span}
      />
    );
  }

  const rawMessages = convertQuerySpanToRawMessages(data);

  return (
    <SpanFieldRender
      attrTos={attrTos}
      messages={rawMessages}
      tagType={tagType}
      span={span}
    />
  );
};

export const QuerySpanDataRender: QuerySpanDataRender = {
  input: (input, attrTos, spanRenderConfig, span) =>
    renderQuerySpanContent(
      input,
      attrTos,
      TagType.Input,
      spanRenderConfig,
      span,
    ),
  output: (output, attrTos, spanRenderConfig, span) =>
    renderQuerySpanContent(
      output,
      attrTos,
      TagType.Output,
      spanRenderConfig,
      span,
    ),
  error: (error?: string, spanRenderConfig?: SpanRenderConfig, span?: Span) => (
    <RawContent
      structuredContent={error ?? ''}
      tagType={TagType.Error}
      span={span}
    />
  ),
};
