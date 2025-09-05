import { I18n } from '@cozeloop/i18n-adapter';
import {
  ExptStatus,
  TurnRunState,
  ItemRunState,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozCheckMarkCircleFill,
  IconCozClockFill,
  IconCozCrossCircleFill,
  IconCozLoading,
  IconCozWarningCircleFill,
} from '@coze-arch/coze-design/icons';

import { type CozeTagColor } from '../types';

/** 实验运行状态信息 */
export interface ExperimentRunStatusInfo {
  name: string;
  status: ExptStatus;
  color: string;
  tagColor: CozeTagColor;
  icon?: React.ReactNode;
  /** 仅用来预览，在筛选中隐藏 */
  hideInFilter?: boolean;
}
/** 实验运行状态信息列表 */
export const experimentRunStatusInfoList: ExperimentRunStatusInfo[] = [
  {
    name: I18n.t('success'),
    status: ExptStatus.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: I18n.t('failure'),
    status: ExptStatus.Failed,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  // {
  //   name: I18n.t('status_failed'),
  //   status: ExptStatus.SystemTerminated,
  //   color: 'red',
  //   tagColor: 'red',
  //   icon: <IconCozCrossCircleFill />,
  // },
  {
    name: I18n.t('in_progress'),
    status: ExptStatus.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: I18n.t('in_progress'),
    status: ExptStatus.Draining,
    color: 'blue',
    tagColor: 'blue',
    hideInFilter: true,
    icon: <IconCozLoading />,
  },
  {
    name: I18n.t('abort'),
    status: ExptStatus.Terminated,
    color: 'orange',
    tagColor: 'yellow',
    icon: <IconCozWarningCircleFill />,
  },
  {
    name: I18n.t('to_be_executed'),
    status: ExptStatus.Pending,
    color: 'grey',
    tagColor: 'primary',
    icon: <IconCozClockFill />,
  },
];

/** 实验单条数据记录运行状态信息 */
export interface ExperimentItemRunStatusInfo {
  name: string;
  status: TurnRunState;
  color: string;
  tagColor: CozeTagColor;
  icon?: React.ReactNode;
}
/** 实验单条数据记录运行状态信息列表 */
export const experimentItemRunStatusInfoList: ExperimentItemRunStatusInfo[] = [
  {
    name: I18n.t('success'),
    status: TurnRunState.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: I18n.t('failure'),
    status: TurnRunState.Fail,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  {
    name: I18n.t('in_progress'),
    status: TurnRunState.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: I18n.t('to_be_executed'),
    status: TurnRunState.Queueing,
    color: 'grey',
    tagColor: 'primary',
    icon: <IconCozClockFill />,
  },
  {
    name: I18n.t('abort'),
    status: TurnRunState.Terminal,
    color: 'orange',
    tagColor: 'yellow',
    icon: <IconCozWarningCircleFill />,
  },
];

/** 实验对话组运行状态信息 */
export interface ExprGroupItemRunStatusInfo {
  name: string;
  status: ItemRunState;
  color: string;
  tagColor: CozeTagColor;
  icon?: React.ReactNode;
}
/** 实验对话组运行状态信息列表 */
export const exprGroupItemRunStatusInfoList: ExprGroupItemRunStatusInfo[] = [
  {
    name: I18n.t('success'),
    status: ItemRunState.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: I18n.t('failure'),
    status: ItemRunState.Fail,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  {
    name: I18n.t('in_progress'),
    status: ItemRunState.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: I18n.t('to_be_executed'),
    status: ItemRunState.Queueing,
    color: 'grey',
    tagColor: 'primary',
    icon: <IconCozClockFill />,
  },
  {
    name: I18n.t('abort'),
    status: ItemRunState.Terminal,
    color: 'orange',
    tagColor: 'yellow',
    icon: <IconCozWarningCircleFill />,
  },
];
