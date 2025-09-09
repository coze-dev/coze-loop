// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { get } from 'lodash-es';
import { OverflowList, Popover, Tag } from '@coze-arch/coze-design';

type OverflowItem = Record<string | number, unknown>;

export function AutoOverflowList<T>({
  items,
  itemRender,
  itemKey = 'id',
  minVisibleItems,
}: {
  items: T[];
  itemRender: (props: {
    item: T;
    index: number;
    inOverflowPopover: boolean;
  }) => React.ReactElement;
  minVisibleItems?: number;
  itemKey?: string | number;
}) {
  const ItemRender = itemRender;
  const visibleItemRenderer = useMemo(
    () => (item: OverflowItem, index: number) => (
      <ItemRender item={item as T} index={index} inOverflowPopover={false} />
    ),
    [],
  );
  return (
    <OverflowList
      itemKey={itemKey}
      items={items as OverflowItem[]}
      visibleItemRenderer={visibleItemRenderer}
      minVisibleItems={minVisibleItems}
      className="flex gap-2"
      overflowRenderer={overflowItems =>
        overflowItems.length > 0 ? (
          <Popover
            position="top"
            content={
              <div className="flex flex-col gap-2 py-2 px-2 text-xs">
                {overflowItems.map((item, index) => (
                  <ItemRender
                    key={get(item, itemKey)}
                    item={item as T}
                    index={index}
                    inOverflowPopover={true}
                  />
                ))}
              </div>
            }
          >
            <Tag
              color="primary"
              className="rounded-2xl h-5 align-top"
              style={{ borderRadius: 12 }}
              onClick={e => e.stopPropagation()}
            >
              +{overflowItems.length}
            </Tag>
          </Popover>
        ) : null
      }
    />
  );
}
