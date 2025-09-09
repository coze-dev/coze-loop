// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState, type ReactNode } from 'react';

import { IconCozMore } from '@coze-arch/coze-design/icons';
import {
  Dropdown,
  Space,
  type SpaceProps,
  type TypographyProps,
  Typography,
  Tooltip,
} from '@coze-arch/coze-design';

import { TooltipWhenDisabled } from '../tooltip-when-disabled';

import styles from './index.module.less';

export interface TableColAction {
  label: ReactNode;
  icon?: ReactNode;
  disabled?: boolean;
  hide?: boolean;
  type?: TypographyProps['type'];
  disabledTooltip?: string;
  onClick?: () => void;
  tooltip?: string;
}

interface Props {
  actions: TableColAction[];
  maxCount?: number;
  disabled?: boolean;
  spaceProps?: SpaceProps;
  wrapperClassName?: string;
  textClassName?: string;
}

export function TableColActions({
  actions,
  maxCount = 2,
  disabled,
  spaceProps = {},
  wrapperClassName = '',
  textClassName = '',
}: Props) {
  const [visible, setVisible] = useState(false);
  const filteredActions = actions.filter(action => !action.hide);
  const firstActions = filteredActions.slice(0, maxCount);
  const moreActions = filteredActions.slice(maxCount);

  return (
    <div
      className={wrapperClassName}
      onClick={e => {
        e.stopPropagation();
      }}
    >
      <Space spacing={12} {...spaceProps}>
        {firstActions.map((action, index) => (
          <TooltipWhenDisabled
            key={index}
            content={action.disabledTooltip || action.label}
            disabled={Boolean(action.disabled)}
          >
            <Typography.Text
              size="small"
              className={`!text-[13px] ${textClassName}`}
              type={action.type}
              disabled={action.disabled ?? disabled}
              onClick={() => {
                if (!(action.disabled ?? disabled)) {
                  action.onClick?.();
                }
              }}
              link={!action.type}
            >
              {action.icon ? null : action.label}
            </Typography.Text>
          </TooltipWhenDisabled>
        ))}
        {moreActions.length > 0 && (
          <Dropdown
            position="bottomLeft"
            visible={visible}
            trigger="custom"
            className={styles.tableColActionsDropdown}
            onClickOutSide={() => setVisible(false)}
            render={
              <Dropdown.Menu mode="menu">
                {moreActions.map((action, index) => {
                  const isDisabled = action.disabled ?? disabled;
                  const disabledTooltipContent = action.disabledTooltip;

                  const dropdownItem = (
                    <Dropdown.Item
                      disabled={isDisabled}
                      onClick={() => {
                        if (!isDisabled) {
                          setVisible(false);
                          action.onClick?.();
                        }
                      }}
                      className="min-w-[90px] !p-0 !pl-2"
                      icon={action.icon}
                      style={{ minWidth: '90px' }}
                    >
                      <Typography.Text
                        type={action.type}
                        size="small"
                        className="!text-[13px] min-w-[80px]"
                        link={!action.type}
                      >
                        {action.label}
                      </Typography.Text>
                    </Dropdown.Item>
                  );

                  return (
                    <div key={index}>
                      {isDisabled && disabledTooltipContent ? (
                        <Tooltip
                          content={disabledTooltipContent}
                          keepDOM={true}
                          theme="dark"
                        >
                          {dropdownItem}
                        </Tooltip>
                      ) : (
                        dropdownItem
                      )}
                    </div>
                  );
                })}
              </Dropdown.Menu>
            }
          >
            <div
              className="flex items-center justify-center"
              onClick={() => setVisible(true)}
            >
              <IconCozMore className="text-[#5A4DED]" />
            </div>
          </Dropdown>
        )}
      </Space>
    </div>
  );
}
