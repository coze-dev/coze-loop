// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  exprGroupItemRunStatusInfoList,
  type ExprGroupItemRunStatusInfo,
} from '@cozeloop/evaluate-components';
import { type ItemRunState } from '@cozeloop/api-schema/evaluation';
import { Space, Tag, Tooltip } from '@coze-arch/coze-design';

const statusMap = exprGroupItemRunStatusInfoList.reduce(
  (prev, item) => ({ ...prev, [item.status]: item }),
  {} as unknown as Record<string | number, ExprGroupItemRunStatusInfo>,
);

/** 实验中单个记录运行状态标签 */
export function ExperimentGroupItemRunStatus({
  status,
  useTag = true,
  onlyIcon,
}: {
  status: ItemRunState | undefined;
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
