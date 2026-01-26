// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { CSS } from '@dnd-kit/utilities';
import { useSortable } from '@dnd-kit/sortable';
import { IconButtonContainer } from '@cozeloop/components';
import { IconCozHandle } from '@coze-arch/coze-design/icons';

import ItemRenderDefault from './item-render-default';

export default function SortableItem<ItemType extends { id: string }>({
  item,
  itemRender,
}: {
  item: ItemType;
  itemRender?: React.FC<{
    item: ItemType;
    action: React.ReactNode;
  }>;
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: item.id });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  };

  const ItemRender = itemRender ?? ItemRenderDefault;
  return (
    <div ref={setNodeRef} style={style} className="relative">
      <ItemRender
        item={item}
        action={
          <IconButtonContainer
            {...attributes}
            {...listeners}
            icon={<IconCozHandle className="cursor-move outline-none" />}
          />
        }
      />
    </div>
  );
}
