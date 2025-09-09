// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type CSSProperties } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';

import styles from './index.module.less';

interface LogicNotProps {
  not?: boolean;
  readonly?: boolean;
  onChange?: (not: boolean) => void;
  className?: string;
  style?: CSSProperties;
}

export function LogicNot(props: LogicNotProps) {
  const { not, readonly, onChange, className, style } = props;
  const onClick = () => {
    if (readonly) {
      return;
    }

    onChange?.(not ? false : true);
  };

  return (
    <div
      className={classNames(
        styles['logic-not'],
        {
          [styles['logic-not_active']]: not,
        },
        className,
      )}
      style={style}
      onClick={e => {
        e.stopPropagation();
        onClick();
      }}
    >
      {I18n.t('logic_expr_not')}
    </div>
  );
}
