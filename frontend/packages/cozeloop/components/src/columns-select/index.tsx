/* eslint-disable @coze-arch/max-line-per-function */
/* eslint-disable @coze-arch/no-batch-import-or-export */
import * as sort from 'react-sortable-hoc';
import { Fragment, useEffect, useMemo, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
// @ts-expect-error react-sortable-hoc ts type issue
const { sortableContainer, sortableElement, sortableHandle } = sort;
const { arrayMove } = sort;

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
  resetButtonText = I18n.t('reset_to_default'),
  className,
  sortable = true,
  itemRender,
  footerRender,
}: ColumnSelectorProps) => {
  const [list, setList] = useState<ColumnItem[]>(() => [...columns]);
  const selectedKeys = useMemo(
    () => list.filter(item => item.checked).map(item => item.key),
    [list],
  );
  const disabledKeys = useMemo(
    () => list.filter(item => item.disabled).map(item => item.key),
    [list],
  );

  const DragHandle = sortableHandle(() => (
    <IconCozHandle
      className="cursor-grab"
      aria-label="拖动排序"
      role="button"
    />
  ));

  const RenderItem = (value: ColumnItem, slot?: React.ReactNode) => {
    // const spanRef = useRef<HTMLSpanElement>(null);
    // const isHovering = useHover(spanRef);
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
            aria-label={`选择${value.value}`}
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
  const SortableContainer = sortableContainer(
    ({ children }: { children: React.ReactNode }) => (
      <div className="max-w-[200px] w-fit rounded-[6px] py-2 px-1 max-h-[372px] overflow-y-auto flex gap-y-1 flex-col">
        {children}
      </div>
    ),
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
                    {resetButtonText}
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
                  {resetButtonText}
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
