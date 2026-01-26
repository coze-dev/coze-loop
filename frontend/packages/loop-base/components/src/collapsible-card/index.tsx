// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties, type ReactNode } from 'react';

import cn from 'classnames';
import { IconCozArrowUpFill } from '@coze-arch/coze-design/icons';
import { Card, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface Props {
  title?: ReactNode;
  children?: ReactNode;
  actionBtns?: ReactNode;
  className?: string;
  style?: CSSProperties;
  hideExpand?: boolean;
  isExpand?: boolean;
  setIsExpand?: (isExpand: boolean) => void;
}

export function CollapsibleCard({
  title,
  children,
  actionBtns,
  className,
  style,
  isExpand,
  setIsExpand,
  hideExpand,
}: Props) {
  return (
    <Card className={cn(styles.card, className)} style={style} bordered={false}>
      {title || actionBtns ? (
        <div className={styles['card-header']}>
          <Typography.Text className={styles['card-title']}>
            {title}
            {children && !hideExpand ? (
              <IconCozArrowUpFill
                className={cn(styles['chevron-icon'], {
                  [styles['chevron-icon-close']]: !isExpand,
                })}
                onClick={() => setIsExpand?.(!isExpand)}
              />
            ) : null}
          </Typography.Text>
          <div>{actionBtns}</div>
        </div>
      ) : null}
      <div
        className={cn(styles['card-content'], {
          [styles.active]: isExpand,
        })}
      >
        {children}
      </div>
    </Card>
  );
}
