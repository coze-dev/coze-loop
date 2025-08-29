import React from 'react';

import { CSVExportStatus } from '@cozeloop/api-schema/evaluation';
import { Button, type ButtonProps } from '@coze-arch/coze-design';

export interface ExportNotificationContentProps {
  status: CSVExportStatus;
  downloadUrl?: string;
  onViewExportRecord?: () => void;
  onDownloadFile?: (url: string) => void;
}

const publicButtonProps: ButtonProps = {
  color: 'secondary',
  size: 'small',
  className: '!px-2 !py-1',
};

const ExportNotificationContent: React.FC<ExportNotificationContentProps> = ({
  status,
  downloadUrl,
  onViewExportRecord,
  onDownloadFile,
}) => {
  const handleDownload = () => {
    if (downloadUrl && onDownloadFile) {
      onDownloadFile(downloadUrl);
    }
  };

  const renderContent = () => {
    const wrapperClassName = 'flex items-center ml-[21px] text-[14px]';
    const buttonNode = (
      <Button {...publicButtonProps} onClick={onViewExportRecord}>
        <span className="text-[#5A4DED] text-[14px]">导出记录</span>
      </Button>
    );

    switch (status) {
      case CSVExportStatus.Running:
        return (
          <div className={wrapperClassName}>
            <span>导出中，查看</span>
            {buttonNode}
          </div>
        );

      case CSVExportStatus.Failed:
        return (
          <div className={wrapperClassName}>
            <span>导出失败</span>
            {buttonNode}
          </div>
        );

      case CSVExportStatus.Success:
        return (
          <div className={`${wrapperClassName} ml-[14px]`}>
            <Button {...publicButtonProps} onClick={handleDownload}>
              <span className="text-[#5A4DED] text-[14px]">下载文件</span>
            </Button>
            {buttonNode}
          </div>
        );

      default:
        return '导出中';
    }
  };

  return <div>{renderContent()}</div>;
};

export default ExportNotificationContent;
