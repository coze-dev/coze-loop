// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @coze-arch/no-batch-import-or-export */
import * as sort from 'react-sortable-hoc';
import { Fragment, useEffect, useMemo, useState } from 'react';

import {
  IconCozHandle,
  IconCozTableSetting,
} from '@coze-arch/coze-design/icons';
import {
  Button,
  Checkbox,
  Dropdown,
  Typography,
  Divider,
  Tooltip,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { useI18n } from '@/provider';
// @ts-expect-error react-sortable-hoc ts type issue
const { sortableContainer, sortableElement, sortableHandle } = sort;
const { arrayMove } = sort;

const SortableContainer = sortableContainer(
  ({ children }: { children: React.ReactNode }) => (
    <div className="max-w-[200px] w-fit rounded-[6px] py-2 px-1 max-h-[372px] overflow-y-auto flex gap-y-1 flex-col">
      {children}
    </div>
  ),
);

const DragHandle = sortableHandle(() => {
  const I18n = useI18n();
  return (
    <IconCozHandle
      className="cursor-grab"
      aria-label={I18n.t('drag_to_sort')}
      role="button"
    />
  );
});
export interface ColumnItem extends ColumnProps {
  key: string;
  value: string;
  disabled?: boolean;
  checked?: boolean;
}

export interface ColumnSelectorProps {
  columns: ColumnItem[];
  onChange?: (items: ColumnItem[]) => void;
  buttonText?: string;
  resetButtonText?: string;
  className?: string;
  sortable?: boolean;
  defaultColumns?: ColumnItem[];
  itemRender?: (item: ColumnItem) => React.ReactNode;
  footerRender?: (item: ColumnItem[]) => React.ReactNode;
}

export const ColumnSelector = ({
  columns,
  defaultColumns = columns,
  onChange,
  buttonText,
  resetButtonText,
  className,
  sortable = true,
  itemRender,
  footerRender,
}: ColumnSelectorProps) => {
  const I18n = useI18n();
  const [list, setList] = useState<ColumnItem[]>(() => [...columns]);
  const selectedKeys = useMemo(
    () => list.filter(item => item.checked).map(item => item.key),
    [list],
  );
  const disabledKeys = useMemo(
    () => list.filter(item => item.disabled).map(item => item.key),
    [list],
  );

  const RenderItem = (value: ColumnItem, slot?: React.ReactNode) => {
    const render = itemRender ? itemRender(value) : null;
    if (render) {
      return render;
    }
    return (
      <span
        // ref={spanRef}
        className="group flex items-center justify-between py-1 px-2 text-[var(--coz-fg-primary)] z-[99999] select-none hover:bg-[var(--coz-mg-secondary)] rounded-[6px] cursor-pointer bg-white"
        style={{
          zIndex: 99999,
        }}
      >
        <div
          className="flex items-center gap-x-2 max-w-full w-full"
          onClick={() => {
            if (disabledKeys.includes(value.key ?? '') || value.disabled) {
              return;
            }
            const newKeys = selectedKeys.includes(value.key ?? '')
              ? selectedKeys.filter(key => key !== value.key)
              : [...selectedKeys, value.key];

            const newColumns = list.map(item => {
              if (newKeys.includes(item.key)) {
                return {
                  ...item,
                  checked: true,
                };
              }

              return {
                ...item,
                checked: false,
              };
            });

            setList(newColumns);
            onChange?.(newColumns);
          }}
        >
          <Checkbox
            disabled={disabledKeys.includes(value.key ?? '') || value.disabled}
            checked={selectedKeys.includes(value.key ?? '')}
            aria-label={`${I18n.t('select')}${value.value}`}
          />

          <Typography.Text
            ellipsis={{
              showTooltip: {
                opts: {
                  content: value.value,
                  theme: 'dark',
                },
              },
            }}
            className="text-[13px] text-[var(--coz-fg-primary)] flex-1 overflow-hidden w-full"
            style={{
              color:
                disabledKeys.includes(value.key ?? '') || value.disabled
                  ? 'var(--coz-fg-dim)'
                  : '',
            }}
          >
            {value.value}
          </Typography.Text>
          <div className="opacity-0 group-hover:opacity-100 transition-opacity flex items-center coz-fg-secondary">
            {slot}
          </div>
        </div>
      </span>
    );
  };

  const SortableItem = sortableElement(({ value }: { value: ColumnItem }) =>
    RenderItem(value, <DragHandle />),
  );

  const handleSortEnd = ({
    oldIndex,
    newIndex,
  }: {
    oldIndex: number;
    newIndex: number;
  }) => {
    const newList = arrayMove(list, oldIndex, newIndex);
    setList(newList);
    onChange?.(newList);
  };

  const handleReset = () => {
    setList(defaultColumns);
    onChange?.(defaultColumns);
  };

  useEffect(() => {
    setList(columns);
  }, [columns]);

  return (
    <div className={className}>
      <Dropdown
        position="bottomRight"
        render={
          <div
            onClick={event => {
              event.stopPropagation();
            }}
          >
            <SortableContainer onSortEnd={handleSortEnd} useDragHandle>
              <>
                {list.map((value, index) =>
                  value?.disabled || !sortable ? (
                    <Fragment key={`item-${value.key}`}>
                      {RenderItem(value)}
                    </Fragment>
                  ) : (
                    <SortableItem
                      key={`item-${value.key}`}
                      index={index}
                      value={value}
                    />
                  ),
                )}
              </>
            </SortableContainer>
            <Divider />
            {footerRender ? (
              <div className="flex items-center">
                {footerRender(list)}
                <Button
                  color="secondary"
                  type="secondary"
                  className="text-center flex-1"
                  onClick={handleReset}
                >
                  <span className="text-brand font-medium text-[13px]">
                    {resetButtonText || I18n.t('reset_to_default')}
                  </span>
                </Button>
              </div>
            ) : (
              <Button
                color="secondary"
                type="secondary"
                className="w-full text-center"
                onClick={handleReset}
              >
                <span className="text-brand font-medium text-[13px]">
                  {resetButtonText || I18n.t('reset_to_default')}
                </span>
              </Button>
            )}
          </div>
        }
        trigger="click"
      >
        <div>
          <Tooltip
            content={I18n.t('column_management')}
            theme="dark"
            position="top"
          >
            <Button
              icon={<IconCozTableSetting />}
              type="primary"
              color="primary"
              className="flex items-center justify-center"
              aria-label={buttonText}
            />
          </Tooltip>
        </div>
      </Dropdown>
    </div>
  );
};
