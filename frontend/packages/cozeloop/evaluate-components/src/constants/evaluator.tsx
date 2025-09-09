// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { EvaluatorRunStatus } from '@cozeloop/api-schema/evaluation';
import {
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';

import { type CozeTagColor } from '../types';

/** 评估器运行状态信息 */
export interface EvaluatorRunStatusInfo {
  name: string;
  status: EvaluatorRunStatus;
  color: string;
  tagColor: CozeTagColor;
  icon?: React.ReactNode;
}
/** 评估器运行状态信息列表 */
export const evaluatorRunStatusInfoList: EvaluatorRunStatusInfo[] = [
  {
    name: I18n.t('success'),
    status: EvaluatorRunStatus.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: I18n.t('failure'),
    status: EvaluatorRunStatus.Fail,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
];
