// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
/* eslint-disable @typescript-eslint/no-explicit-any */
import { isEmpty } from 'lodash-es';

import { safeJsonParse } from '@/shared/utils/json';
import { type Span, TagType } from '@/features/trace-data/types';

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

export const getSpanContentField = (span: Span) => {
  const { input, output } = span;
  const { error } = span.custom_tags ?? {};
  const tools = (safeJsonParse(input) as any)?.tools;
  const reasonStr = parseReasoningContent(output);

  return [
    {
      content: error,
      title: 'Error',
      tagType: TagType.Error,
    },
    {
      content: tools,
      title: 'Tools',
      tagType: TagType.Functions,
    },
    {
      content: input,
      title: 'Input',
      tagType: TagType.Input,
    },
    {
      content: reasonStr,
      title: 'Reasoning Content',
      tagType: TagType.ReasoningContent,
    },
    {
      content: output,
      title: 'Output',
      tagType: TagType.Output,
    },
  ].filter(field => !isEmpty(field.content));
};

export const getPartUrl = (url?: string, attrTos?: Span['attr_tos']) => {
  if (!url) {
    return null;
  }
  // http or https
  if (url.startsWith('http') || url.startsWith('https')) {
    return url;
  }
  // base64 image
  if (url.startsWith('data:image/')) {
    return url;
  }
  // tos key -> url
  return attrTos?.multimodal_data?.[url] || url;
};
