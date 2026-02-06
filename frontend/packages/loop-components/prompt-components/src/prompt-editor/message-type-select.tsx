// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import cn from 'classnames';
import { type Role } from '@cozeloop/api-schema/prompt';
import { IconCozNodeExpand } from '@coze-arch/coze-design/icons';
import { Button, Menu, Typography } from '@coze-arch/coze-design';

import styles from './index.module.less';

interface MessageTypeSelectProps<R = Role> {
  value: R;
  onChange?: (v: R) => void;
  disabled?: boolean;
  messageTypeList?: Array<{ label: string; value: R }>;
}

export function MessageTypeSelect<R extends string | number = Role>({
  value,
  onChange,
  disabled,
  messageTypeList = [],
}: MessageTypeSelectProps<R>) {
  const [visible, setVisible] = useState(false);
  if (disabled) {
    return (
      <Typography.Text
        size="small"
        type="tertiary"
        className={cn('px-2', styles['role-display'], 'variable-text')}
      >
        {(messageTypeList.find(it => it.value === value)?.label || value) ??
          '-'}
      </Typography.Text>
    );
  }

  return (
    <Menu
      visible={visible}
      trigger="custom"
      position="bottomLeft"
      showTick={false}
      render={
        <Menu.SubMenu
          mode="selection"
          selectedKeys={[`${value}`]}
          onSelectionChange={v => {
            onChange?.(v as R);
            setVisible(false);
          }}
        >
          {messageTypeList?.map(it => (
            <Menu.Item
              itemKey={`${it.value}`}
              key={it.value}
              className={cn('!px-2', {
                'coz-mg-primary': `${it}` === `${value}`,
              })}
            >
              <Typography.Text className="variable-text">
                {it.label}
              </Typography.Text>
            </Menu.Item>
          ))}
        </Menu.SubMenu>
      }
      onClickOutSide={() => setVisible(false)}
    >
      <Button
        size="mini"
        color="secondary"
        onClick={() => setVisible(true)}
        className={cn(styles['role-display'], 'variable-text')}
      >
        {(messageTypeList.find(it => it.value === value)?.label || value) ??
          '-'}
        <IconCozNodeExpand className="ml-0.5" />
      </Button>
    </Menu>
  );
}
