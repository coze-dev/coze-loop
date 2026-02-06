// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cls from 'classnames';

import { useLocale } from '@/i18n';

import styles from './index.module.less';

interface LogicNotProps {
  not?: boolean;
  readonly?: boolean;
  onChange?: (not: boolean) => void;
  className?: string;
  style?: React.CSSProperties;
}

export function LogicNot(props: LogicNotProps) {
  const { t } = useLocale();
  const { not, readonly, onChange, className, style } = props;
  const onClick = () => {
    if (readonly) {
      return;
    }

    onChange?.(not ? false : true);
  };

  return (
    <div
      className={cls(
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
      {t('logic_expr_not')}
    </div>
  );
}
