// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import classNames from 'classnames';

import styles from './index.module.less';

export interface CardPaneProps extends React.HTMLAttributes<HTMLDivElement> {
  /* hover 的时候是否展示 shadow 样式 */
  hoverShadow?: boolean;
}

export function CardPane({ hoverShadow, className, ...props }: CardPaneProps) {
  return (
    <div
      className={classNames(
        styles.container,
        {
          [styles.shadow]: hoverShadow,
        },
        className,
      )}
      {...props}
    />
  );
}
