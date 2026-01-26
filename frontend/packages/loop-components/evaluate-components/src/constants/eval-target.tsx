// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { EvalTargetRunStatus } from '@cozeloop/api-schema/evaluation';

import { type CozeTagColor } from '../types';

/** 评测对象运行状态信息 */
export interface EvalTargetRunStatusInfo {
  name: string;
  status: EvalTargetRunStatus;
  color: string;
  tagColor: CozeTagColor;
}
/** 评测对象运行状态信息列表 */
export const evalTargetRunStatusInfoList: EvalTargetRunStatusInfo[] = [
  {
    name: I18n.t('success'),
    status: EvalTargetRunStatus.Success,
    color: 'green',
    tagColor: 'green',
  },
  {
    name: I18n.t('failure'),
    status: EvalTargetRunStatus.Fail,
    color: 'red',
    tagColor: 'red',
  },
];
