// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, useEffect } from 'react';

import cn from 'classnames';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Collapse } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface CollapseCardProps {
  className?: string;
  title: React.ReactNode;
  children: React.ReactNode;
  subInfo?: React.ReactNode;
  extra?: React.ReactNode;
  defaultVisible?: boolean;
  visible?: boolean;
  disableCollapse?: boolean;
  onVisibleChange?: (visible: boolean) => void;
}

export const CollapseCard = ({
  className,
  title,
  children,
  subInfo,
  extra,
  defaultVisible,
  visible,
  onVisibleChange,
  disableCollapse,
}: CollapseCardProps) => {
  const [activeKey, setActiveKey] = useState(defaultVisible ? ['1'] : []);

  // 处理受控模式
  useEffect(() => {
    if (visible !== undefined) {
      setActiveKey(visible ? ['1'] : []);
    }
  }, [visible]);

  const customHeader = (
    <div className="flex items-center justify-between flex-1">
      <div className="flex items-center gap-2">
        {title}
        <IconCozArrowDown
          className={cn(styles['chevron-icon'], {
            [styles['chevron-icon-close']]: !activeKey.length,
          })}
        />
        {subInfo}
      </div>
      {extra}
    </div>
  );

  const handleChange = (keys: string[]) => {
    if (visible === undefined) {
      setActiveKey(keys);
    }
    onVisibleChange?.(keys.length > 0);
  };

  if (disableCollapse) {
    return (
      <div className={cn('flex flex-col gap-4', className)}>
        {title}
        {children}
      </div>
    );
  }

  return (
    <Collapse activeKey={activeKey} onChange={v => handleChange(v as string[])}>
      <Collapse.Panel
        className={cn(styles['coze-up-panel'], styles['coze-up-panel-hidden'], {
          className,
        })}
        header={customHeader}
        itemKey="1"
        showArrow={false}
        extra={extra}
      >
        {children}
      </Collapse.Panel>
    </Collapse>
  );
};
