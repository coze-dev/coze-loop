// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type span } from '@cozeloop/api-schema/observation';

import { type RemoveUndefinedOrString } from '@/features/trace-data/types/utils';
import {
  type RawMessage,
  TagType,
  type Span,
} from '@/features/trace-data/types';
import { type SpanRenderConfig } from '@/features/trace-data';

import { type Output } from '../model';
import { SpanFieldRender } from '../../components/span-field-render';

interface Props {
  output: RemoveUndefinedOrString<Output>;
  attrTos: Span['attr_tos'];
  spanRenderConfig?: SpanRenderConfig;
  span?: Span;
  context?: span.OutputSpan[] | undefined;
}

export const ModelOutput = (props: Props) => {
  const { output, attrTos, span } = props;
  return (
    <SpanFieldRender
      attrTos={attrTos}
      messages={output.choices.reduce((acc, cur) => {
        acc.push(cur.message);
        return acc;
      }, [] as RawMessage[])}
      tagType={TagType.Output}
      span={span}
    />
  );
};
