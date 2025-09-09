// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { LogicExpr } from '@cozeloop/components';
import { IconCozFilter, IconCozInfoCircle } from '@coze-arch/coze-design/icons';
import {
  Button,
  Divider,
  Popover,
  Tooltip,
  type PopoverProps,
} from '@coze-arch/coze-design';

import { type LogicFilter, type LogicField } from './logic-types';
import RightRender from './logic-right-render';
import OperatorRender from './logic-operator-render';
import LeftRender from './logic-left-render';

import styles from './logic-editor.module.less';

const getValidFilterFields = (value?: LogicFilter) => {
  if (!value?.exprs) {
    return {
      exprs: [],
    };
  }

  const isEmpty = (val: string | undefined | null | string[]) =>
    val === undefined ||
    val === '' ||
    val === null ||
    (Array.isArray(val) && val.length === 0);

  // 左中右 三元都存在才有效
  const validArray = value.exprs.filter(
    // 由于暂时不包含为空不为空的case，所以这里的判断条件要求左中右都必须存在
    exp => !isEmpty(exp.left) && !isEmpty(exp.operator) && !isEmpty(exp.right),
  );

  return {
    exprs: validArray,
  };
};
/** 逻辑筛选器 */
// eslint-disable-next-line @coze-arch/max-line-per-function, complexity
export default function LogicEditor({
  fields = [],
  disabled = false,
  value,
  popoverProps = {},
  tooltip,
  clearEmptyCondition = true,
  enableCascadeMode = false,
  onChange,
  onConfirm,
  onClose,
}: {
  fields: LogicField[];
  disabled?: boolean;
  value?: LogicFilter | undefined;
  popoverProps?: PopoverProps;
  tooltip?: React.ReactNode;
  /** 是否过滤空条件 */
  clearEmptyCondition?: boolean;
  /** 字段选择开启级联模式 */
  enableCascadeMode?: boolean;
  onChange?: (val?: LogicFilter) => void;
  onConfirm?: (val?: LogicFilter) => void;
  onClose?: () => void;
}) {
  const [logicFilter, setLogicFilter] = useState<LogicFilter | undefined>(
    value,
  );
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    setLogicFilter(value);
  }, [value]);

  const logicExpr = (
    <LogicExpr
      defaultExpr={{
        left: '',
        operator: '',
        right: '',
      }}
      value={logicFilter}
      onChange={val => {
        setLogicFilter(val);
        onChange?.(val);
      }}
      leftRender={renderProps => (
        <LeftRender
          {...renderProps}
          fields={fields}
          disabled={disabled}
          enableCascadeMode={enableCascadeMode}
        />
      )}
      operatorRender={renderProps => (
        <OperatorRender
          {...renderProps}
          fields={fields}
          disabled={disabled}
          enableCascadeMode={enableCascadeMode}
        />
      )}
      rightRender={renderProps => (
        <RightRender
          {...renderProps}
          fields={fields}
          disabled={disabled}
          enableCascadeMode={enableCascadeMode}
        />
      )}
      allowLogicOperators={['and']}
      logicToggleReadonly={true}
      maxNestingDepth={1}
    />
  );
  const hasMultiExpr =
    Array.isArray(logicFilter?.exprs) && logicFilter.exprs.length > 1;
  const popoverContentConatienr = (
    <div
      className={classNames(
        'flex flex-col py-3 gap-3 text-[13px]',
        enableCascadeMode ? 'w-[640px]' : 'w-[620px]',
        styles['expr-logic-editor'],
      )}
    >
      <div className="flex items-center px-4">
        <div className="font-medium">{I18n.t('data_filter')}</div>
        {tooltip ? (
          <Tooltip theme="dark" content={tooltip}>
            <IconCozInfoCircle className="ml-1 text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
          </Tooltip>
        ) : null}
        <div
          className="ml-auto cursor-pointer font-medium text-[var(--coz-fg-secondary)] hover:text-[rgb(var(--coze-up-brand-9))]"
          onClick={() => {
            setLogicFilter(undefined);
            onChange?.(undefined);
          }}
        >
          {I18n.t('clear_filter')}
        </div>
      </div>
      <div className={hasMultiExpr ? '' : 'pl-3 pr-2'}>
        <div>{logicExpr}</div>
      </div>
      <Divider />
      <div className="flex justify-end px-4">
        <Button
          color="brand"
          onClick={() => {
            let logicFilterData = logicFilter;
            if (clearEmptyCondition) {
              logicFilterData = getValidFilterFields(logicFilter);
              setLogicFilter(logicFilterData);
            }
            onConfirm?.(logicFilterData);
            setVisible(false);
            onClose?.();
          }}
        >
          {I18n.t('application')}
        </Button>
      </div>
    </div>
  );
  const count = value?.exprs?.length ?? 0;
  return (
    <Popover
      trigger="click"
      position="bottomLeft"
      {...popoverProps}
      style={{ padding: 0, ...(popoverProps.style ?? {}) }}
      visible={visible}
      content={popoverContentConatienr}
      onVisibleChange={newVisible => {
        setVisible(newVisible);
        if (!newVisible) {
          // 关闭时放弃本次变更，恢复为上层设置的 value
          setLogicFilter(value);
          onClose?.();
        }
      }}
    >
      <Button icon={<IconCozFilter />} color="primary">
        {I18n.t('data_filter')}
        {count ? (
          <div className="flex items-center justify-center w-5 h-5 rounded-[50%] text-brand-9 bg-brand-4 ml-1 text-[13px]">
            {count}
          </div>
        ) : null}
      </Button>
    </Popover>
  );
}
