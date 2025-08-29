import classNames from 'classnames';
import { IconCozRefresh } from '@coze-arch/coze-design/icons';
import { Button, Space, Tooltip, type ButtonProps } from '@coze-arch/coze-design';

import { ColumnSelector, type ColumnSelectorProps } from '@/columns-select';

import styles from './index.module.less';

export interface TableHeaderProps {
  columnSelectorConfigProps?: ColumnSelectorProps;
  filterForm?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  spaceProps?: Record<string, unknown>;
  refreshButtonPros?: ButtonProps;
}

export function TableHeader({
  columnSelectorConfigProps,
  filterForm,
  actions,
  className,
  style,
  spaceProps,
  refreshButtonPros,
}: TableHeaderProps) {
  return (
    <div
      className={classNames('flex flex-row justify-between w-full ', className)}
      style={style}
    >
      <div className={classNames('flex flex-row', styles['table-header-form'])}>
        {filterForm}
      </div>
      <Space {...(spaceProps || {})}>
        {refreshButtonPros ? (
          <Tooltip content="刷新" theme="dark">
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
