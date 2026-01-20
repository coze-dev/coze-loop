// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import {
  Button,
  Space,
  Tooltip,
  type ButtonProps,
} from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import {
  TableColsConfig,
  type TableColsConfigProps,
} from '../table-cols-config';
import { ColumnSelector, type ColumnSelectorProps } from '../columns-select';

import styles from './index.module.less';

export interface TableHeaderProps {
  columnSelectorConfigProps?: ColumnSelectorProps;
  tableColsConfigProps?: TableColsConfigProps;
  filterForm?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  spaceProps?: Record<string, unknown>;
  refreshButtonPros?: ButtonProps;
}

export function TableHeader({
  columnSelectorConfigProps,
  tableColsConfigProps,
  filterForm,
  actions,
  className,
  style,
  spaceProps,
  refreshButtonPros,
}: TableHeaderProps) {
  const I18n = useI18n();
  return (
    <div
      className={classNames('flex flex-row justify-between w-full ', className)}
      style={style}
    >
      <div className={classNames('flex flex-row', styles['table-header-form'])}>
        {tableColsConfigProps ? (
          <TableColsConfig
            className={classNames('mr-2', tableColsConfigProps.className)}
            {...tableColsConfigProps}
          />
        ) : null}
        {filterForm}
      </div>
      <Space {...(spaceProps || {})}>
        {refreshButtonPros ? (
          <Tooltip content={I18n.t('refresh')} theme="dark">
            <Button
              color="primary"
              icon={<IconCozRefresh />}
              {...refreshButtonPros}
            ></Button>
          </Tooltip>
        ) : null}
        {columnSelectorConfigProps ? (
          <ColumnSelector
            className={classNames(columnSelectorConfigProps.className)}
            {...columnSelectorConfigProps}
          />
        ) : null}
        {actions}
      </Space>
    </div>
  );
}
