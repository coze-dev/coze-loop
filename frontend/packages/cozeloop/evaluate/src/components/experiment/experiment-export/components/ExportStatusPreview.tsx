// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type CozeTagColor } from '@cozeloop/evaluate-components';
import {
  CSVExportStatus,
  type ExptResultExportRecord,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozCheckMarkCircleFillPalette,
  IconCozCrossCircleFill,
  IconCozLoading,
  IconCozInfoCircleFill,
} from '@coze-arch/coze-design/icons';
import { Tag, Tooltip } from '@coze-arch/coze-design';

interface ExportStatusInfo {
  status: CSVExportStatus;
  text: string;
  tagColor: CozeTagColor;
  icon?: React.ReactNode;
  className?: string;
}
export const exportStatusInfoList: ExportStatusInfo[] = [
  {
    status: CSVExportStatus.Success,
    text: I18n.t('success'),
    tagColor: 'green',
    icon: <IconCozCheckMarkCircleFillPalette />,
  },
  {
    status: CSVExportStatus.Running,
    text: I18n.t('in_progress'),
    tagColor: 'blue',
    icon: <IconCozLoading />,
  },
  {
    status: CSVExportStatus.Failed,
    text: I18n.t('failure'),
    tagColor: 'red',
    icon: <IconCozCrossCircleFill />,
  },
];

export function ExportStatusPreview({
  exportRecord,
  // estimatedTime,
  // errorMessage,
}: {
  exportRecord: ExptResultExportRecord;
  // estimatedTime?: string;
  // errorMessage?: string;
}) {
  const { csv_export_status: status, error } = exportRecord;
  const ruleTypeMap = useMemo(
    () =>
      exportStatusInfoList.reduce(
        (acc, cur) => ({ ...acc, [cur.status]: cur }),
        {} as unknown as Record<CSVExportStatus, ExportStatusInfo>,
      ),
    [],
  );

  const errorMessage = useMemo(() => {
    if (status === CSVExportStatus.Failed && error?.detail) {
      return (
        <Tooltip content={error.detail} theme="dark">
          <div className="flex items-center h-full h-[20px] w-[20px]">
            <IconCozInfoCircleFill color="var(--coz-fg-secondary)" />
          </div>
        </Tooltip>
      );
    }
    return null;
  }, [status, error]);

  if (!status || !ruleTypeMap[status]) {
    return '-';
  }

  const statusInfo = ruleTypeMap[status];

  return (
    <div className="flex items-center gap-2">
      <Tag
        className={`${statusInfo?.className || ''} text-[12px]`}
        prefixIcon={statusInfo.icon}
        color={statusInfo?.tagColor}
        style={{ fontWeight: 500 }}
      >
        {statusInfo?.text}
      </Tag>
      {errorMessage}
      {/* 预计时间 */}
      {/* {estimatedTime ? (
        <span className="ml-2 text-xs font-large text-center text-[color:var(--text-color-text-2,#42464E)]">
          {estimatedTime}
        </span>
      ) : null} */}
      {/* 错误 */}
      {/* {errorMessage ? (
        <span className="ml-2 text-xs font-large text-center text-[color:var(--text-color-text-2,#42464E)]">
          {errorMessage}
        </span>
      ) : null} */}
    </div>
  );
}
