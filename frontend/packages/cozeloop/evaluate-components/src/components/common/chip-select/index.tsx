// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import cs from 'classnames';
import { Select, type SelectProps } from '@coze-arch/coze-design';

import styles from './index.module.less';
export const ChipSelect = (props: SelectProps) => (
  <Select {...props} className={cs(styles.select, props.className)} />
);
