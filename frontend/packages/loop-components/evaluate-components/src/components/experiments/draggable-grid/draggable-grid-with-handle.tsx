// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  rectSortingStrategy,
} from '@dnd-kit/sortable';
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  DragOverlay,
  type DragStartEvent,
  type DragEndEvent,
} from '@dnd-kit/core';
import { IconButtonContainer } from '@cozeloop/components';
import { IconCozHandle } from '@coze-arch/coze-design/icons';

import SortableItem from './sortable-item';
import ItemRenderDefault from './item-render-default';

export interface ItemRenderProps<ItemType> {
  item: ItemType;
  action: React.ReactNode;
}

export default function DraggableGrid<ItemType extends { id: string }>({
  items,
  columnCount = 3,
  itemRender,
  onItemsChange,
}: {
  items: ItemType[];
  columnCount?: number;
  itemRender?: React.FC<ItemRenderProps<ItemType>>;
  onItemsChange?: (
    list: ItemType[] | ((items: ItemType[]) => ItemType[]),
  ) => void;
}) {
  const [activeId, setActiveId] = useState<string | number | null>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );

  function handleDragStart(event: DragStartEvent) {
    setActiveId(event.active.id);
  }

  function handleDragEnd(event: DragEndEvent) {
    const { active, over } = event;

    if (active.id !== over?.id) {
      onItemsChange?.((oldItems: ItemType[]) => {
        const oldIndex = oldItems.findIndex(item => item.id === active.id);
        const newIndex = oldItems.findIndex(item => item.id === over?.id);

        return arrayMove(oldItems, oldIndex, newIndex);
      });
    }

    setActiveId(null);
  }

  function handleDragCancel() {
    setActiveId(null);
  }

  const activeItem = items.find(item => item.id === activeId);

  const ItemRender = itemRender ?? ItemRenderDefault;
  return (
    <div>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        onDragCancel={handleDragCancel}
      >
        <SortableContext items={items} strategy={rectSortingStrategy}>
          <div
            className="grid grid-cols-3 gap-4"
            style={{
              gridTemplateColumns: `repeat(${columnCount}, minmax(0, 1fr))`,
            }}
          >
            {items.map(item => (
              <SortableItem key={item.id} item={item} itemRender={itemRender} />
            ))}
          </div>
        </SortableContext>
        <DragOverlay adjustScale={true}>
          {activeId && activeItem ? (
            <ItemRender
              item={activeItem}
              action={
                <IconButtonContainer
                  icon={<IconCozHandle className="cursor-move outline-none" />}
                />
              }
            />
          ) : null}
        </DragOverlay>
      </DndContext>
    </div>
  );
}
