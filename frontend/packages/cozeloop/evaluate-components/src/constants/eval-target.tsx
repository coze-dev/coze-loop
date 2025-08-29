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
    name: '成功',
    status: EvalTargetRunStatus.Success,
    color: 'green',
    tagColor: 'green',
  },
  {
    name: '失败',
    status: EvalTargetRunStatus.Fail,
    color: 'red',
    tagColor: 'red',
  },
];
