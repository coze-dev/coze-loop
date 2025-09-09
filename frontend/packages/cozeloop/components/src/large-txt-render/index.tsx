// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef } from 'react';

import { useInfiniteScroll } from 'ahooks';

const PER_PRE_MAX_LENGTH = 25000;

export interface LargeTxtRenderProps {
  text: string;
}

export const LargeTxtRender = (props: LargeTxtRenderProps) => {
  const { text } = props;

  const ref = useRef<HTMLPreElement>(null);

  const getMoreTextChunk = (startIndex: number) => {
    const endIndex = startIndex + PER_PRE_MAX_LENGTH;
    const chunk = text.slice(startIndex, endIndex);
    const hasMore = endIndex < text.length;

    return {
      list: [chunk],
      nextId: hasMore ? endIndex.toString() : undefined,
    };
  };

  const { data } = useInfiniteScroll(
    d => {
      const startIndex = d ? parseInt(d.nextId) : 0;
      return Promise.resolve(getMoreTextChunk(startIndex));
    },
    {
      target: ref,
      isNoMore: d => d?.nextId === undefined,
    },
  );

  return (
    <pre
      ref={ref}
      className="m-0 break-words whitespace-pre-wrap max-h-full overflow-y-auto text-[13px] leading-4 text-[var(--coz-fg-primary)] font-normal"
    >
      {data?.list.join('') || ''}
    </pre>
  );
};
