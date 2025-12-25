// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';
import { Select } from '@coze-arch/coze-design';

import { useLocale } from '@/i18n';

import type { LogicOperator } from './types';
import { LogicNot } from './logic-not';
import { LOGIC_OPTIONS } from './consts';
import { ReactComponent as SelectIcon } from './assets/select.svg';

import styles from './index.module.less';

interface LogicToggleProps {
  enableNot?: boolean;
  operator?: LogicOperator;
  not?: boolean;
  hideLine?: boolean;
  readonly?: boolean;
  allowLogicOperators?: LogicOperator[];
  onChange: (operator: LogicOperator, not: boolean) => void;
}

export function LogicToggle(props: LogicToggleProps) {
  const { t } = useLocale();
  const {
    operator = 'and',
    hideLine,
    enableNot,
    not = false,
    readonly,
    allowLogicOperators,
    onChange,
  } = props;

  const filteredLogicOptions = allowLogicOperators
    ? LOGIC_OPTIONS.filter(op =>
        allowLogicOperators.includes(op.value as LogicOperator),
      )
    : LOGIC_OPTIONS;

  const label =
    (filteredLogicOptions.find(op => op.value === operator)?.label as string) ||
    '-';

  return (
    <div
      className={cls(styles['logic-toggle'], {
        [styles['logic-toggle_and']]: operator === 'and',
        [styles['logic-toggle_or']]: operator === 'or',
        [styles['logic-toggle_hide-line']]: hideLine,
      })}
    >
      {readonly ? (
        <div className={styles['logic-toggle-tag']}>{t(label)}</div>
      ) : (
        <Select
          size="small"
          showClear={false}
          dropdownClassName={styles['logic-toggle-select-dropdown']}
          triggerRender={() => (
            <div
              className={cls(
                styles['logic-toggle-tag'],
                styles['logic-toggle-tag_edit'],
              )}
            >
              {enableNot ? (
                <LogicNot
                  not={not}
                  readonly={readonly}
                  className={styles['logic-toggle-tag-not']}
                  onChange={val => {
                    onChange(operator, val);
                  }}
                />
              ) : null}
              {t(label)}
              <SelectIcon className={styles['logic-toggle-tag-icon']} />
            </div>
          )}
          optionList={filteredLogicOptions.map(item => ({
            ...item,
            label: t(item.label),
          }))}
          value={operator}
          onChange={val => {
            if (!val) {
              return;
            }
            onChange(val as LogicOperator, not);
          }}
        />
      )}
    </div>
  );
}
