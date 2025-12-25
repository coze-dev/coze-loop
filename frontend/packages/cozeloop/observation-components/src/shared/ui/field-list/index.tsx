// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { handleCopy as copy } from '@cozeloop/components';
import { Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

export interface Field {
  key: string;
  title: React.ReactNode;
  item: React.ReactNode;
  enableCopy?: boolean;
}

interface FieldListProps extends React.HTMLAttributes<HTMLDivElement> {
  fields: Field[];
  minColWidth?: number;
  maxColNum?: number;
}
export const FieldList = ({
  fields,
  minColWidth,
  maxColNum,
  className,
  style,
}: FieldListProps) => (
  <div
    className={cls(styles['field-list'], className)}
    style={{
      gap: 4,
      gridTemplateColumns: `repeat(auto-fill, minmax(max(${
        minColWidth ?? 240
      }px, calc((100% - 30px) / ${maxColNum ?? 3})), 1fr))`,
      ...style,
    }}
  >
    {fields.map(field => {
      const { key, title, item, enableCopy } = field;
      return (
        <div
          className="justify-start flex-1 min-w-0 overflow-hidden inline-flex items-center text-xs"
          key={key}
        >
          <span className="text-[#1d1c2359] whitespace-nowrap flex items-center text-xs">
            {title}&nbsp;:&nbsp;
          </span>
          <span className="overflow-hidden text-xs max-w-full">
            {typeof item === 'string' ? (
              <Typography.Text
                style={{
                  fontSize: 12,
                  cursor: enableCopy ? 'copy' : undefined,
                  width: '100%',
                }}
                ellipsis={{
                  showTooltip: {
                    opts: {
                      theme: 'dark',
                    },
                  },
                }}
                onClick={() => enableCopy && copy(item)}
              >
                {item}
              </Typography.Text>
            ) : (
              <span className="flex items-center text-xs">{item}</span>
            )}
          </span>
        </div>
      );
    })}
  </div>
);
