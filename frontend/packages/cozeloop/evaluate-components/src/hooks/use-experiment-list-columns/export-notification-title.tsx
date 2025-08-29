import React from 'react';

import { CSVExportStatus } from '@cozeloop/api-schema/evaluation';
import {
  IconCozLoading,
  IconCozCheckMarkCircleFill,
  IconCozCrossCircleFill,
} from '@coze-arch/coze-design/icons';
import { Loading } from '@coze-arch/coze-design';

export interface ExportNotificationTitleProps {
  status: CSVExportStatus;
  taskId?: string;
}

const ExportNotificationTitle: React.FC<ExportNotificationTitleProps> = ({
  status,
  taskId,
}) => {
  const getIcon = () => {
    switch (status) {
      case CSVExportStatus.Running:
        return (
          <Loading
            loading={true}
            size="mini"
            color="blue"
            style={{ marginRight: '8px' }}
          />
        );
      case CSVExportStatus.Success:
        return (
          <IconCozCheckMarkCircleFill
            style={{ color: '#52c41a', marginRight: '8px' }}
          />
        );
      case CSVExportStatus.Failed:
        return (
          <IconCozCrossCircleFill
            style={{ color: '#D0292F', marginRight: '8px' }}
          />
        );
      default:
        return (
          <IconCozLoading style={{ color: '#1890ff', marginRight: '8px' }} />
        );
    }
  };

  const getTitle = () => {
    switch (status) {
      case CSVExportStatus.Running:
        return '实验明细导出中';
      case CSVExportStatus.Failed:
        return '实验明细导出失败';
      case CSVExportStatus.Success:
        return '实验明细导出成功';
      default:
        return '实验明细导出中';
    }
  };

  const getTaskInfo = () => {
    if (taskId) {
      return ` #${taskId}`;
    }
    return '';
  };

  return (
    <span className="flex items-center">
      <span className="mt-[-1px] h-[14px]">{getIcon()}</span>
      <span>
        {getTitle()}
        {getTaskInfo()}
      </span>
    </span>
  );
};

export default ExportNotificationTitle;
