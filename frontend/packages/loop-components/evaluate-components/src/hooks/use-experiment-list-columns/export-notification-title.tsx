// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
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
        return I18n.t('cozeloop_open_evaluate_experiment_details_exporting');
      case CSVExportStatus.Failed:
        return I18n.t(
          'cozeloop_open_evaluate_experiment_details_export_failed',
        );
      case CSVExportStatus.Success:
        return I18n.t(
          'cozeloop_open_evaluate_experiment_details_export_success',
        );
      default:
        return I18n.t('cozeloop_open_evaluate_experiment_details_exporting');
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
