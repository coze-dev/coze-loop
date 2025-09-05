import { useEffect, useMemo, useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  downloadExptExportFile,
  formateTime,
  TypographyText,
} from '@cozeloop/evaluate-components';
import {
  type ColumnItem,
  dealColumnsWithStorage,
  TableColActions,
  UserProfile,
} from '@cozeloop/components';
import {
  type ExptResultExportRecord,
  CSVExportStatus,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';

import { ExportStatusPreview } from './components/ExportStatusPreview';

// 导出任务操作组件
function ExportTaskRowActions({
  exportRecord,
  setModalLoading,
  source,
}: {
  exportRecord: ExptResultExportRecord;
  setModalLoading: (loading: boolean) => void;
  source?: string;
}) {
  const { csv_export_status } = exportRecord;
  const inProgress = csv_export_status === CSVExportStatus.Running;
  const isFailed = csv_export_status === CSVExportStatus.Failed;
  const isExpired = exportRecord.expired;

  const popoverText = useMemo(() => {
    if (inProgress) {
      return I18n.t('export_in_progress_wait');
    }
    if (isFailed) {
      return I18n.t('export_failed_retry');
    }
    // 过期
    if (isExpired) {
      return I18n.t('export_file_expired_100_days_limit');
    }
    return null;
  }, [inProgress, isExpired, isFailed]);

  const actionsArr = [
    // 下载操作 - 只有成功状态才可下载
    {
      label: I18n.t('download'),
      disabledTooltip: popoverText ?? undefined,
      disabled: inProgress || isExpired || isFailed,
      onClick: async () => {
        setModalLoading(true);
        sendEvent(EVENT_NAMES.cozeloop_experiment_export_download, {
          from: source,
        });
        const res = await StoneEvaluationApi.GetExptResultExportRecord({
          workspace_id: exportRecord.workspace_id,
          expt_id: exportRecord.expt_id,
          export_id: exportRecord.export_id,
        });
        const downUrl = res?.expt_result_export_records?.URL;
        if (downUrl) {
          downloadExptExportFile(downUrl, String(exportRecord?.export_id));
        }
        setModalLoading(false);
      },
    },
  ].filter(Boolean);

  return (
    <TableColActions
      textClassName="h-full content-center"
      wrapperClassName="py-2.5 px-5 h-full"
      spaceProps={{
        className: '!p-0',
      }}
      actions={actionsArr}
      maxCount={1}
    />
  );
}

export function getExportExperimentColumns() {
  const newColumns: ColumnItem[] = [
    {
      title: I18n.t('export_task_id'),
      value: I18n.t('export_task_id'),
      disableColumnManage: true,
      dataIndex: 'export_id',
      key: 'export_id',
      fixed: 'left',
      align: 'left',
      width: 220,
      checked: true,
      disabled: true,
      render: (text: string) => <TypographyText>{text}</TypographyText>,
    },
    {
      title: I18n.t('export_status'),
      value: I18n.t('export_status'),
      disableColumnManage: true,
      dataIndex: 'csv_export_status',
      key: 'csv_export_status',
      width: 120,
      checked: true,
      render: (_, record: ExptResultExportRecord) => (
        <ExportStatusPreview exportRecord={record} />
      ),
    },
    {
      title: I18n.t('export_format'),
      value: I18n.t('export_format'),
      dataIndex: 'export_type',
      key: 'export_type',
      width: 100,
      checked: true,
      render: () => <TypographyText>csv</TypographyText>,
    },
    {
      title: I18n.t('operator'),
      value: I18n.t('application_operator'),
      dataIndex: 'base_info',
      key: 'base_info',
      width: 220,
      checked: true,
      render: (_: string, record: ExptResultExportRecord) => (
        <UserProfile
          avatarUrl={record.base_info?.created_by?.avatar_url ?? ''}
          name={record.base_info?.created_by?.name ?? ''}
        />
      ),
    },
    {
      title: I18n.t('start_time'),
      value: I18n.t('start_time'),
      dataIndex: 'start_time',
      key: 'start_time',
      width: 200,
      render: (text: string) => (
        <TypographyText>{formateTime(text)}</TypographyText>
      ),
    },
    {
      title: I18n.t('completion_time'),
      value: I18n.t('data_annotate_end_time'),
      dataIndex: 'end_time',
      key: 'end_time',
      width: 200,
      render: (text: string) => (
        <TypographyText>{formateTime(text)}</TypographyText>
      ),
    },
  ];
  return newColumns;
}

export function useExportExperimentListColumns({
  columnManageStorageKey,
  setModalLoading,
  source,
}: {
  columnManageStorageKey: string;
  setModalLoading: (loading: boolean) => void;
  source?: string;
}) {
  const [columns, setColumns] = useState<ColumnItem[]>([]);

  useEffect(() => {
    const newColumns = getExportExperimentColumns();

    const actionsColumn: ColumnItem = {
      title: () => (
        <div style={{ padding: '5px 20px' }}>{I18n.t('operation')}</div>
      ),
      value: I18n.t('operation'),
      disableColumnManage: true,
      dataIndex: 'action',
      key: 'action',
      fixed: 'right',
      align: 'right',
      width: 80,
      checked: true,
      disabled: true,
      className: '!p-0',
      render: (_: unknown, record: ExptResultExportRecord) => (
        <ExportTaskRowActions
          exportRecord={record}
          setModalLoading={setModalLoading}
          source={source}
        />
      ),
    };

    const newColumnItems = dealColumnsWithStorage(
      columnManageStorageKey,
      newColumns,
    );
    setColumns([...newColumnItems, actionsColumn]);
  }, [columnManageStorageKey, setModalLoading]);

  return {
    columns,
    setColumns,
  };
}
