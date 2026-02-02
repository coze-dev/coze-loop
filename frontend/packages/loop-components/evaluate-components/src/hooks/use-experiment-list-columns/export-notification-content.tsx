// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
        <span className="text-[#5A4DED] text-[14px]">
          {I18n.t('evaluate_export_records')}
        </span>
      </Button>
    );

    switch (status) {
      case CSVExportStatus.Running:
        return (
          <div className={wrapperClassName}>
            <span>
              {I18n.t('cozeloop_open_evaluate_export_in_progress_view')}
            </span>
            {buttonNode}
          </div>
        );

      case CSVExportStatus.Failed:
        return (
          <div className={wrapperClassName}>
            <span>{I18n.t('evaluate_export_failed')}</span>
            {buttonNode}
          </div>
        );

      case CSVExportStatus.Success:
        return (
          <div className={`${wrapperClassName} ml-[14px]`}>
            <Button {...publicButtonProps} onClick={handleDownload}>
              <span className="text-[#5A4DED] text-[14px]">
                {I18n.t('cozeloop_open_evaluate_download_file')}
              </span>
            </Button>
            {buttonNode}
          </div>
        );

      default:
        return I18n.t('model_exporting');
    }
  };

  return <div>{renderContent()}</div>;
};

export default ExportNotificationContent;
