import { useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  fetchExportStatus,
  useGlobalEvalConfig,
} from '@cozeloop/evaluate-components';
import { TooltipWhenDisabled } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import {
  type Experiment,
  ExptResultExportType,
  ExptStatus,
} from '@cozeloop/api-schema/evaluation';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Button, Dropdown } from '@coze-arch/coze-design';

import ExportTableModal from './export-table-modal';

interface ExportMenuProps {
  experiment?: Experiment;
  source?: string;
}

const ExportMenu = (props: ExportMenuProps) => {
  const { experiment, source } = props;
  const [exportModalVisible, setExportModalVisible] = useState(false);
  const { spaceID } = useSpace();
  const { ExptExportDropdownButton } = useGlobalEvalConfig();

  // 终态: 成功和失败
  const isFinished =
    experiment?.status === ExptStatus.Success ||
    experiment?.status === ExptStatus.Failed;

  // csv_export_status
  const onExportCSV = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_export_click, {
      from: source,
      type: 'csv',
    });
    fetchExportStatus(
      {
        workspace_id: spaceID.toString(),
        expt_id: experiment?.id ?? '',
        export_type: ExptResultExportType.CSV,
      },
      () => setExportModalVisible(true),
      experiment, // 传入完整的 experiment 对象
      source,
    );
  };

  const onViewDownloadFiles = () => {
    sendEvent(EVENT_NAMES.cozeloop_experiment_export_record_click, {
      from: source,
    });
    setExportModalVisible(true);
  };

  const defaultDropdown = ExptExportDropdownButton ? (
    // 如果有配置的 ExptExportDropdownButton 组件，直接使用
    <ExptExportDropdownButton
      experiment={experiment}
      onExportModalOpen={() => setExportModalVisible(true)}
      onExportCSV={onExportCSV}
      onViewDownloadFiles={onViewDownloadFiles}
    />
  ) : (
    <Dropdown
      position="bottomLeft"
      render={
        <Dropdown.Menu>
          <Dropdown.Title className="!pl-2">
            {I18n.t('export_data')}
          </Dropdown.Title>
          <TooltipWhenDisabled
            theme="dark"
            disabled={!isFinished}
            content={
              !isFinished
                ? I18n.t('export_not_supported_incomplete_experiment')
                : undefined
            }
          >
            <Dropdown.Item
              className="!pl-2"
              onClick={onExportCSV}
              disabled={!isFinished}
            >
              <span
                style={{
                  color: !isFinished
                    ? 'rgba(var(--coze-fg-1), var(--coze-fg-1-alpha)) !important'
                    : '',
                }}
              >
                {I18n.t('csv_format')}
              </span>
            </Dropdown.Item>
          </TooltipWhenDisabled>
          <Dropdown.Title className="!pl-2">
            {I18n.t('export_records')}
          </Dropdown.Title>
          <Dropdown.Item className="!pl-2" onClick={onViewDownloadFiles}>
            {I18n.t('view_and_download_files')}
          </Dropdown.Item>
        </Dropdown.Menu>
      }
    >
      <Button color="primary" iconPosition="right" icon={<IconCozArrowDown />}>
        {I18n.t('export')}
      </Button>
    </Dropdown>
  );

  return (
    <>
      {defaultDropdown}
      <ExportTableModal
        experiment={experiment}
        visible={exportModalVisible}
        setVisible={setExportModalVisible}
        source={source}
      />
    </>
  );
};

export default ExportMenu;
