// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { getSpanContentField } from '@/features/trace-data/utils/span';
import { type Span } from '@/features/trace-data/types';

import { SpanContentContainer } from './span-content-container';
import { RawContent } from './raw-content';

interface SpanContentDetailProps {
  span: Span;
}
export const SpanContentDetail = (props: SpanContentDetailProps) => {
  const { span } = props;
  const spanDetailList = getSpanContentField(span);
  return (
    <>
      {spanDetailList.map((spanDetail, index) => (
        <SpanContentContainer
          key={spanDetail.title + index}
          content={spanDetail.content}
          title={spanDetail.title}
          tagType={spanDetail.tagType}
          span={span}
          hasBottomLine={index !== spanDetailList.length - 1}
          copyConfig={{
            moduleName: 'trace',
            point: 'span',
          }}
          children={(_renderType, content) => (
            <RawContent
              structuredContent={content}
              tagType={spanDetail.tagType}
              attrTos={span.attr_tos}
              span={span}
            />
          )}
        />
      ))}
    </>
  );
};
