// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ReactNode, useEffect, useState, useMemo } from 'react';
import React from 'react';

import classNames from 'classnames';
import { sleep } from '@cozeloop/toolkit';
import { IconCozSetting, IconCozEye } from '@coze-arch/coze-design/icons';
import { Button, SideSheet, Switch, Typography } from '@coze-arch/coze-design';

import { useI18n } from '@/provider';

import { generateColumnsWithKey, generateConfigColumns } from './util';
import { useHiddenColKeys } from './use-hidden-col-keys';
import { type ColumnPropsPro, type ColKey } from './type';

export { type ColumnPropsPro, type ColKey };

import styles from './index.module.less';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export interface TableColsConfigProps<T extends Record<string, any> = any> {
  className?: string;
  defaultHiddenColKeys?: ColKey[];
  localStorageKey?: string;

  columns?: ColumnPropsPro<T>[];
  onChangeConfig?: (newColumns: ColumnPropsPro<T>[]) => void;
  onClose?: () => void;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function TableColsConfig<T extends Record<string, any> = any>({
  className,
  localStorageKey,
  defaultHiddenColKeys,
  columns,
  onChangeConfig,
  onClose,
}: TableColsConfigProps<T>) {
  const I18n = useI18n();
  const [hiddenColKeys, setHiddenColKeys] = useHiddenColKeys({
    localStorageKey,
    defaultHiddenColKeys,
  });
  const [visible, setVisible] = useState(false);
  const handledColumns = useMemo(
    () => generateColumnsWithKey(columns),
    [columns],
  );

  useEffect(() => {
    if (!visible) {
      const configColumns = generateConfigColumns(
        handledColumns,
        hiddenColKeys,
      );
      if (configColumns) {
        // sleep 100ms，防止页面卡顿
        sleep(100)
          .then(() => {
            onChangeConfig?.(configColumns);
          })
          .catch(console.error);
      }
    }
  }, [handledColumns, visible]);

  const renderItem = ({
    record,
    index,
    disable,
    parent,
  }: {
    record: ColumnPropsPro<T>;
    index: number;
    disable?: boolean;
    parent?: ColumnPropsPro<T>;
  }) => {
    const { title, children, configurable = true, colKey } = record;

    const hidden = Boolean(colKey && hiddenColKeys.includes(colKey));

    return (
      <React.Fragment key={index}>
        <div
          className={classNames(styles.row, {
            [styles['row-border']]: parent || index !== 0,
          })}
        >
          <div className={classNames(styles.cell, styles['cell-left'])}>
            <Typography.Text ellipsis={{ rows: 1, showTooltip: true }}>
              {title as ReactNode}
            </Typography.Text>
          </div>
          <div className={classNames(styles.cell, styles['cell-right'])}>
            {!configurable || !colKey ? null : (
              <Switch
                size="small"
                disabled={disable}
                checked={!hidden}
                onChange={v => {
                  const colsSet = new Set(hiddenColKeys);
                  if (v) {
                    colsSet.delete(colKey);
                  } else {
                    colsSet.add(colKey);
                  }
                  const newList = [...colsSet];
                  setHiddenColKeys(newList);
                }}
              />
            )}
          </div>
        </div>
        {children?.length ? (
          <div className="ml-4">
            {children.map((item, idx) =>
              renderItem({
                record: item,
                index: idx,
                disable: hidden,
                parent: record,
              }),
            )}
          </div>
        ) : null}
      </React.Fragment>
    );
  };

  return (
    <>
      <Button
        className={className}
        icon={<IconCozSetting />}
        color="primary"
        onClick={() => setVisible(true)}
      >
        {I18n.t('column_configuration')}
      </Button>
      <SideSheet
        placement="right"
        size="small"
        visible={visible}
        onCancel={() => {
          setVisible(false);
          onClose?.();
        }}
      >
        <div className={styles.row}>
          <div
            className={classNames(
              styles.cell,
              styles['cell-left'],
              styles['cell-thead'],
            )}
          >
            {I18n.t('column_name')}
          </div>
          <div
            className={classNames(
              styles.cell,
              styles['cell-right'],
              styles['cell-thead'],
            )}
          >
            {<IconCozEye style={{ lineHeight: '22px' }} />}
          </div>
        </div>
        {handledColumns?.map((item, index) =>
          renderItem({ record: item, index }),
        )}
        <div className="h-5" />
      </SideSheet>
    </>
  );
}
