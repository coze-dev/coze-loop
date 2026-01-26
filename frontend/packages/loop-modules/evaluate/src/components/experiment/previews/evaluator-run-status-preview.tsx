// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import {
  evaluatorRunStatusInfoList,
  type EvaluatorRunStatusInfo,
} from '@cozeloop/evaluate-components';
import { type EvaluatorRunStatus } from '@cozeloop/api-schema/evaluation';
import { Space, Tag, Tooltip } from '@coze-arch/coze-design';

const statusMap = evaluatorRunStatusInfoList.reduce(
  (prev, item) => ({ ...prev, [item.status]: item }),
  {} as unknown as Record<string | number, EvaluatorRunStatusInfo>,
);

/** 实验中单个记录运行状态标签 */
export default function EvaluatorRunStatusPreview({
  status,
  useTag = true,
  onlyIcon,
  extra,
  className,
}: {
  status: EvaluatorRunStatus | undefined;
  useTag?: boolean;
  onlyIcon?: boolean;
  extra?: React.ReactNode;
  className?: string;
}) {
  const statusInfo = statusMap[status ?? ''];
  if (statusInfo) {
    const content = onlyIcon ? (
      <Tooltip content={statusInfo.name} theme="dark">
        {statusInfo.icon}
      </Tooltip>
    ) : (
      <Space spacing={4}>
        {statusInfo.icon}
        {statusInfo.name}
      </Space>
    );
    const statusContent = useTag ? (
      <Tag
        className="shrink-0"
        size={onlyIcon ? 'mini' : 'small'}
        color={statusInfo.tagColor}
      >
        {content}
      </Tag>
    ) : (
      <span className="shrink-0" style={{ color: statusInfo.color }}>
        {content}
      </span>
    );
    return (
      <div
        className={classNames(
          'flex items-center overflow-hidden gap-1',
          className,
        )}
      >
        {statusContent} {extra}
      </div>
    );
  }
  return '-';
}
