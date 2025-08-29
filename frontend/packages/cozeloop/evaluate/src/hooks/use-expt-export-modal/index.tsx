import { useState, useCallback } from 'react';

import { type ExptResultExportRecord } from '@cozeloop/api-schema/evaluation';

import ExportTableModal from '@/components/experiment/experiment-export/export-table-modal';

export const useExptExportModal = () => {
  const [visible, setVisible] = useState(false);
  const [currentExperiment, setCurrentExperiment] =
    useState<ExptResultExportRecord>();

  // 导出记录按钮的点击处理函数
  const onExportRecordClick = useCallback((record: ExptResultExportRecord) => {
    setCurrentExperiment(record);
    setVisible(true);
  }, []);

  // 导出记录的表格列配置
  const exportRecordColumn = {
    title: '导出记录',
    key: 'export_record',
    width: 80,
    render: (_: unknown, record: ExptResultExportRecord) => (
      <span
        className="cursor-pointer text-primary hover:text-primary-hover"
        onClick={() => onExportRecordClick(record)}
      >
        导出记录
      </span>
    ),
  };

  // 弹窗组件
  const ExportModalNode = () => (
    <ExportTableModal
      visible={visible}
      setVisible={setVisible}
      experiment={currentExperiment}
    />
  );

  return {
    ExportModalNode,
    exportRecordColumn,
  };
};
