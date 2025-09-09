// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  experimentItemRunStatusInfoList,
  type ExperimentItemRunStatusInfo,
} from '@cozeloop/evaluate-components';
import { type TurnRunState } from '@cozeloop/api-schema/evaluation';
import { Space, Tag, Tooltip } from '@coze-arch/coze-design';

const statusMap = experimentItemRunStatusInfoList.reduce(
  (prev, item) => ({ ...prev, [item.status]: item }),
  {} as unknown as Record<string | number, ExperimentItemRunStatusInfo>,
);

/** 实验中单个记录运行状态标签 */
export default function ExperimentItemRunStatus({
  status,
  useTag = true,
  onlyIcon,
}: {
  status: TurnRunState | undefined;
  useTag?: boolean;
  onlyIcon?: boolean;
}) {
  const statusInfo = statusMap[status ?? ''];
  if (statusInfo) {
    const content = onlyIcon ? (
      statusInfo.icon
    ) : (
      <Space spacing={4}>
        {statusInfo.icon}
        {statusInfo.name}
      </Space>
    );
    return useTag ? (
      <Tooltip content={statusInfo.name} theme="dark">
        <Tag size={'small'} className="align-top" color={statusInfo.tagColor}>
          {content}
        </Tag>
      </Tooltip>
    ) : (
      <span style={{ color: statusInfo.color }}>{content}</span>
    );
  }
  return '-';
}
