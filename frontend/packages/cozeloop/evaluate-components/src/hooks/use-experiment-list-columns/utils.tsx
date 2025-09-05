/* eslint-disable max-params */
import React from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { UserProfile } from '@cozeloop/components';
import {
  ExptRetryMode,
  type UserInfo,
  type Experiment,
  type ExportExptResultRequest,
  type ExptResultExportType,
  type GetExptResultExportRecordRequest,
  CSVExportStatus,
} from '@cozeloop/api-schema/evaluation';
import { StoneEvaluationApi } from '@cozeloop/api-schema';
import {
  Modal,
  Notification,
  Select,
  Tag,
  type ColumnProps,
} from '@coze-arch/coze-design';

import { formateTime } from '../../utils';
import { TypographyText } from '../../components/text-ellipsis';
import { EvaluateTargetTypePreview } from '../../components/previews/evaluate-target-type-preview';
import { EvalTargetPreview } from '../../components/previews/eval-target-preview';
import { EvaluationSetPreview } from '../../components/previews/eval-set-preview';
import { ExperimentRunStatus } from '../../components/experiments/previews/experiment-run-status';
import LoopTableSortIcon from '../../components/dataset-list/sort-icon';
import ExportNotificationTitle from './export-notification-title';
import ExportNotificationContent from './export-notification-content';
import ExperimentEvaluatorAggregatorScore from './experiment-evaluator-aggregator-score';

/** 实验列表列配置 */

export function getExperimentColumns({
  spaceID,
  enableSort = false,
}: {
  spaceID: Int64;
  enableSort?: boolean;
  onRefresh?: () => void;
}) {
  const columns: ColumnProps<Experiment>[] = [
    {
      title: I18n.t('experiment_name'),
      disableColumnManage: true,
      dataIndex: 'name',
      key: 'name',
      width: 200,
      render: text => <TypographyText>{text}</TypographyText>,
    },
    {
      title: I18n.t('evaluation_object_type'),
      dataIndex: 'type',
      key: 'type',
      width: 120,
      render(_, record) {
        return (
          <EvaluateTargetTypePreview
            type={record.eval_target?.eval_target_type}
          />
        );
      },
    },
    {
      title: I18n.t('evaluation_object'),
      dataIndex: 'eval_target',
      key: 'eval_target',
      width: 240,
      render(val, record) {
        if (!val) {
          return (
            <EvaluationSetPreview
              evalSet={record.eval_set}
              enableLinkJump={true}
              jumpBtnClassName={'show-in-table-row-hover'}
            />
          );
        }
        return (
          <div className="flex items-center">
            <div className="min-w-0">
              <EvalTargetPreview
                spaceID={spaceID}
                evalTarget={val}
                enableLinkJump={true}
                showIcon={true}
                jumpBtnClassName={'show-in-table-row-hover'}
              />
            </div>
            {record.target_runtime_param?.json_value &&
            record.target_runtime_param?.json_value !== '{}' ? (
              <Tag color="grey" className="shrink-0 hide-in-table-row-hover">
                {I18n.t('cozeloop_open_evaluate_dynamic_parameters')}
              </Tag>
            ) : null}
          </div>
        );
      },
    },
    {
      title: I18n.t('associated_evaluation_set'),
      dataIndex: 'eval_set',
      key: 'eval_set',
      width: 215,
      render: val => (
        <EvaluationSetPreview
          evalSet={val}
          enableLinkJump={true}
          jumpBtnClassName={'show-in-table-row-hover'}
        />
      ),
    },
    {
      title: I18n.t('status'),
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (_, record: Experiment) => (
        <div onClick={e => e.stopPropagation()}>
          <ExperimentRunStatus
            status={record.status}
            experiment={record}
            enableOnClick={false}
            showProcess={true}
          />
        </div>
      ),
    },
    {
      title: I18n.t('score'),
      dataIndex: 'score',
      key: 'score',
      width: 330,
      render: (_, record: Experiment) => (
        <ExperimentEvaluatorAggregatorScore
          evaluators={record.evaluators ?? []}
          spaceID={spaceID}
          evaluatorAggregateResult={
            record.expt_stats?.evaluator_aggregate_results
          }
        />
      ),
    },
    {
      title: I18n.t('description'),
      dataIndex: 'desc',
      key: 'desc',
      width: 160,
      render: val => <TypographyText>{val || '-'}</TypographyText>,
    },
    {
      title: I18n.t('creator'),
      dataIndex: 'base_info.created_by',
      key: 'create_by',
      width: 160,
      render: (val: UserInfo) =>
        val?.name ? (
          <UserProfile avatarUrl={val?.avatar_url} name={val?.name} />
        ) : (
          '-'
        ),
    },
    {
      title: I18n.t('create_time'),
      dataIndex: 'start_time',
      key: 'start_time',
      width: 180,
      sorter: enableSort,
      sortIcon: LoopTableSortIcon,
      render: val => formateTime(val),
    },
    {
      title: I18n.t('end_time'),
      dataIndex: 'end_time',
      key: 'end_time',
      width: 180,
      render: val => formateTime(val),
    },
  ];
  return columns;
}

export function handleDelete({
  record,
  spaceID,
  onRefresh,
}: {
  record: Experiment;
  spaceID: Int64;
  onRefresh?: () => void;
}) {
  Modal.confirm({
    title: I18n.t('delete_experiment'),
    content: (
      <>
        {I18n.t('cozeloop_open_evaluate_confirm_to_delete')}
        <span className="font-medium px-[2px]">{record.name}</span>
        {I18n.t('data_engine_this_change_irreversible')}
      </>
    ),
    okText: I18n.t('delete'),
    cancelText: I18n.t('cancel'),
    okButtonColor: 'red',
    width: 420,
    autoLoading: true,
    async onOk() {
      if (record.id) {
        await StoneEvaluationApi.DeleteExperiment({
          workspace_id: spaceID,
          expt_id: record.id,
        });
        onRefresh?.();
      }
    },
  });
}

export function handleRetry({
  record,
  spaceID,
  onRefresh,
}: {
  record: Experiment;
  spaceID: Int64;
  onRefresh?: () => void;
}) {
  Modal.confirm({
    title: I18n.t('retry_experiment'),
    content: I18n.t('only_re_evaluate_failed_part'),
    okText: I18n.t('confirm'),
    cancelText: I18n.t('cancel'),
    width: 420,
    autoLoading: true,
    async onOk() {
      await StoneEvaluationApi.RetryExperiment({
        workspace_id: spaceID,
        expt_id: record.id ?? '',
        retry_mode: ExptRetryMode.RetryAll,
      });
      onRefresh?.();
    },
  });
}

export function handleCopy({
  record,
  onOk,
}: {
  record: Experiment;
  onOk: () => void;
}) {
  Modal.confirm({
    title: I18n.t('copy_experiment_config'),
    content: (
      <>
        {I18n.t('evaluate_copy')}
        <span className="font-medium px-[2px]">{record.name}</span>
        {I18n.t('cozeloop_open_evaluate_config_and_launch_experiment')}
      </>
    ),
    okText: I18n.t('confirm'),
    cancelText: I18n.t('cancel'),
    width: 420,
    onOk,
  });
}

const downloadFile = (url: string) => {
  const link = document.createElement('a');
  link.href = url;
  link.target = '_blank';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};

/** 导出状态存储 */
const exportStorageKey = 'expt_export_status';
export const getExportStatus = () =>
  window.localStorage.getItem(exportStorageKey);
export const setExportStatus = (exptId: string) =>
  window.localStorage.setItem(exportStorageKey, exptId);

export const clearExportStatus = () =>
  window.localStorage.removeItem(exportStorageKey);

/** 实验表格导出相关函数&组件 */
// 简化的通知创建函数，使用共享组件
function createExportNotification(options: {
  status: CSVExportStatus;
  taskId?: string;
  id?: string;
  onClose?: () => void;
  downloadUrl?: string;
  onViewExportRecord?: () => void;
  source?: string;
}) {
  const {
    status,
    taskId,
    id,
    onClose,
    downloadUrl,
    onViewExportRecord,
    source,
  } = options;

  const handleDownloadFile = (url: string) => {
    // 下载文件埋点
    sendEvent(EVENT_NAMES.cozeloop_experiment_export_download, {
      from: `${source}_notification`,
    });
    downloadFile(url);
  };

  return Notification.open({
    title: React.createElement(ExportNotificationTitle, { status, taskId }),
    content: React.createElement(ExportNotificationContent, {
      status,
      downloadUrl,
      onViewExportRecord,
      onDownloadFile: handleDownloadFile,
    }),
    duration: 0,
    id,
    onClose,
  });
}

const pollingExportStatus = async (
  params: GetExptResultExportRecordRequest,
  prevId: string,
  timer?: NodeJS.Timeout,
  onOpenExportModal?: (experiment: Experiment) => void,
  taskId?: string,
  experiment?: Experiment,
  source?: string,
) => {
  const result = await StoneEvaluationApi.GetExptResultExportRecord({
    workspace_id: params.workspace_id,
    expt_id: params.expt_id,
    export_id: params.export_id,
  });

  const exportRecord = result?.expt_result_export_records;

  const status = exportRecord?.csv_export_status;

  const exportError = exportRecord?.error;

  const onViewExportRecord =
    onOpenExportModal && experiment
      ? () => onOpenExportModal(experiment)
      : undefined;

  if (exportError) {
    createExportNotification({
      status: CSVExportStatus.Failed,
      taskId,
      id: prevId,
      onViewExportRecord,
      source,
    });
    clearExportStatus();
    return;
  }

  // 1. 导出成功
  if (status === CSVExportStatus.Success) {
    createExportNotification({
      status: CSVExportStatus.Success,
      taskId,
      id: prevId,
      downloadUrl: exportRecord?.URL,
      onViewExportRecord,
      source,
    });
    clearExportStatus();
    return;
  } else if (status === CSVExportStatus.Running) {
    // 2. 导出中
    createExportNotification({
      status: CSVExportStatus.Running,
      taskId,
      id: prevId,
      onClose: () => {
        clearTimeout(timer);
      },
      onViewExportRecord,
      source,
    });
    // 2s 轮询
    timer = setTimeout(() => {
      pollingExportStatus(
        params,
        prevId,
        timer,
        onOpenExportModal,
        taskId,
        experiment,
        source,
      );
    }, 2000);
  } else if (status === CSVExportStatus.Failed) {
    // 3. 导出失败
    createExportNotification({
      status: CSVExportStatus.Failed,
      taskId,
      id: prevId,
      onViewExportRecord,
      source,
    });
    clearExportStatus();
  }
};

export const fetchExportStatus = async (
  params: ExportExptResultRequest,
  onOpenExportModal?: (experiment: Experiment) => void,
  experiment?: Experiment,
  source?: string,
) => {
  // 1. 创建导出任务
  const taskId = Date.now().toString();

  // 使用新的通知创建函数显示初始状态
  const initId = createExportNotification({
    status: CSVExportStatus.Running,
    taskId,
    onViewExportRecord:
      onOpenExportModal && experiment
        ? () => onOpenExportModal(experiment)
        : undefined,
    source,
  });

  // 2. 触发导出
  const res = await StoneEvaluationApi.ExportExptResult({
    workspace_id: params.workspace_id,
    expt_id: params.expt_id,
    export_type: params.export_type,
  });

  let timer: NodeJS.Timeout | undefined;

  pollingExportStatus(
    {
      workspace_id: params.workspace_id,
      expt_id: params.expt_id,
      export_id: res.export_id.toString(),
    },
    initId,
    timer,
    onOpenExportModal,
    taskId,
    experiment,
    source,
  );
};

export function handleExport({
  record,
  spaceID,
  onOpenExportModal,
  source,
}: {
  record: Experiment;
  spaceID: Int64;
  onOpenExportModal?: (experiment: Experiment) => void;
  source?: string;
}) {
  let selectedFormat = 'csv'; // 默认选择CSV格式

  Modal.confirm({
    title: I18n.t('cozeloop_open_evaluate_export_experiment_details'),
    content: (
      <div className="pt-4">
        <div className="mb-2">
          <label className="text-sm font-medium">
            {I18n.t('evaluate_export_format')}
            <span className="text-red-500">*</span>
          </label>
        </div>
        <Select
          placeholder={I18n.t('please_select', { field: '' })}
          defaultValue="csv"
          style={{ width: '100%' }}
          onChange={value => {
            selectedFormat = value as string;
          }}
        >
          <Select.Option value="csv">CSV</Select.Option>
          {/* <Select.Option value="zip">Zip</Select.Option> */}
        </Select>
      </div>
    ),
    okText: I18n.t('evaluate_export'),
    cancelText: I18n.t('global_btn_cancel'),
    width: 420,
    autoLoading: true,
    onOk() {
      if (!selectedFormat) {
        throw new Error(
          I18n.t('cozeloop_open_evaluate_please_select_export_format'),
        );
      }

      sendEvent(EVENT_NAMES.cozeloop_experiment_export_click, {
        from: source,
        type: selectedFormat,
      });

      // 这里调用导出函数 - 参考 export-menu.tsx 的实现
      fetchExportStatus(
        {
          workspace_id: spaceID.toString(),
          expt_id: record.id ?? '',
          export_type: selectedFormat as ExptResultExportType,
        },
        onOpenExportModal,
        record, // 传入完整的 experiment 对象
        source,
      );
    },
  });
}

export function handleExportRecord({
  record,
  onOpenExportModal,
}: {
  record: Experiment;
  onOpenExportModal?: (experiment: Experiment) => void;
}) {
  // 如果提供了自定义的打开弹窗函数，打开弹窗
  if (onOpenExportModal) {
    onOpenExportModal(record);
    return;
  }
}
