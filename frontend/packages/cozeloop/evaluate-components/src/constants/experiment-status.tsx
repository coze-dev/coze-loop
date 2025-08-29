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
    name: '成功',
    status: ExptStatus.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: '失败',
    status: ExptStatus.Failed,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  // {
  //   name: '失败',
  //   status: ExptStatus.SystemTerminated,
  //   color: 'red',
  //   tagColor: 'red',
  //   icon: <IconCozCrossCircleFill />,
  // },
  {
    name: '进行中',
    status: ExptStatus.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: '进行中',
    status: ExptStatus.Draining,
    color: 'blue',
    tagColor: 'blue',
    hideInFilter: true,
    icon: <IconCozLoading />,
  },
  {
    name: '中止',
    status: ExptStatus.Terminated,
    color: 'orange',
    tagColor: 'yellow',
    icon: <IconCozWarningCircleFill />,
  },
  {
    name: '待执行',
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
    name: '成功',
    status: TurnRunState.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: '失败',
    status: TurnRunState.Fail,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  {
    name: '进行中',
    status: TurnRunState.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: '待执行',
    status: TurnRunState.Queueing,
    color: 'grey',
    tagColor: 'primary',
    icon: <IconCozClockFill />,
  },
  {
    name: '中止',
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
    name: '成功',
    status: ItemRunState.Success,
    color: 'green',
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFill />,
  },
  {
    name: '失败',
    status: ItemRunState.Fail,
    color: 'red',
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
  {
    name: '进行中',
    status: ItemRunState.Processing,
    color: 'blue',
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    name: '待执行',
    status: ItemRunState.Queueing,
    color: 'grey',
    tagColor: 'primary',
    icon: <IconCozClockFill />,
  },
  {
    name: '中止',
    status: ItemRunState.Terminal,
    color: 'orange',
    tagColor: 'yellow',
    icon: <IconCozWarningCircleFill />,
  },
];
