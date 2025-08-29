/* eslint-disable @coze-arch/max-line-per-function */
import { useEffect, useRef, useState } from 'react';

import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { GuardPoint, useGuards, GuardActionType } from '@cozeloop/guard';
import {
  TableColActions,
  IDRender,
  type TableColAction,
} from '@cozeloop/components';
import {
  useCurrentEnterpriseId,
  useNavigateModule,
} from '@cozeloop/biz-hooks-adapter';
import { ExptStatus } from '@cozeloop/api-schema/evaluation';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { Tooltip, type ColumnProps } from '@coze-arch/coze-design';

import { useGlobalEvalConfig } from '@/stores/eval-global-config';

import { dealColumnsFromStorage } from '../../components/common';
import {
  getExperimentColumns,
  handleDelete,
  handleRetry,
  handleCopy,
  handleExport,
  handleExportRecord,
} from './utils';

// 导出通知相关组件和工具函数
export { default as ExportNotificationTitle } from './export-notification-title';
export { default as ExportNotificationContent } from './export-notification-content';

function isExperimentFail(status: ExptStatus | undefined) {
  return [
    ExptStatus.Failed,
    ExptStatus.SystemTerminated,
    ExptStatus.Terminated,
  ].includes(status as ExptStatus);
}

export interface UseExperimentListColumnsProps {
  spaceID: Int64;
  /** 开启ID列，默认为true */
  enableIdColumn?: boolean;
  /** 开启操作列，默认为true  */
  enableActionColumn?: boolean;
  /** 开启列排序，默认为false */
  enableSort?: boolean;
  /** 表格行操作显示控制，默认为true显示 */
  actionVisibleControl?: {
    copy?: boolean;
    retry?: boolean;
    delete?: boolean;
  };
  /** 详情跳转的来源路径（在实验详情页面点击返回跳转的路径） */
  detailJumpSourcePath?: string;
  columnManageStorageKey?: string;
  onRefresh?: () => void;
  onDetailClick?: (e: Experiment) => void;
  extraShrinkActions?: TableColAction[];
  /** 自定义导出记录弹窗处理函数 */
  onOpenExportModal?: (experiment: Experiment) => void;
  /** 导出来源 */
  source?: string;
}

const PERSONAL_ENTERPRISE_ID = 'personal';

/** 实验列表列配置 */
export function useExperimentListColumns({
  spaceID,
  enableIdColumn = true,
  enableActionColumn = true,
  enableSort = false,
  columnManageStorageKey,
  detailJumpSourcePath,
  actionVisibleControl,
  extraShrinkActions = [],
  onRefresh,
  onDetailClick,
  onOpenExportModal,
  source,
}: UseExperimentListColumnsProps) {
  const guards = useGuards({
    points: [
      GuardPoint['eval.experiments.copy'],
      GuardPoint['eval.experiments.delete'],
      GuardPoint['eval.experiments.retry'],
    ],
  });

  const navigate = useNavigateModule();

  const [columns, setColumns] = useState<ColumnProps[]>([]);
  const [defaultColumns, setDefaultColumns] = useState<ColumnProps[]>([]);

  const currentEnterpriseId = useCurrentEnterpriseId();
  const isPersonalEnterprise = currentEnterpriseId === PERSONAL_ENTERPRISE_ID;

  const copyGuardType = guards.data[GuardPoint['eval.experiments.copy']].type;
  const retryGuardType = guards.data[GuardPoint['eval.experiments.retry']].type;
  const deleteGuardType =
    guards.data[GuardPoint['eval.experiments.delete']].type;

  const guardsRef = useRef(guards);
  guardsRef.current = guards;

  const { TableExportActionButton } = useGlobalEvalConfig();

  const handleRetryOnCLick = (record: Experiment) => {
    const action = () => {
      handleRetry({ record, spaceID, onRefresh });
    };
    if (retryGuardType === GuardActionType.GUARD) {
      guardsRef.current.data[GuardPoint['eval.experiments.retry']].preprocess(
        action,
      );
    } else {
      action();
    }
  };

  const handleDetailOnClick = (record: Experiment) => {
    onDetailClick?.(record);
    navigate(
      `evaluation/experiments/${record.id}`,
      detailJumpSourcePath
        ? { state: { from: detailJumpSourcePath } }
        : undefined,
    );
  };

  const handleCopyOnClick = (record: Experiment) => {
    const action = () => {
      handleCopy({
        record,
        onOk: () => {
          navigate(
            `evaluation/experiments/create?copy_experiment_id=${record.id}`,
          );
        },
      });
    };

    if (copyGuardType === GuardActionType.GUARD) {
      guardsRef.current.data[GuardPoint['eval.experiments.copy']].preprocess(
        action,
      );
    } else {
      action();
    }
  };

  useEffect(() => {
    const actionsColumn: ColumnProps<Experiment> = {
      title: '操作',
      disableColumnManage: true,
      dataIndex: 'action',
      key: 'action',
      fixed: 'right',
      align: 'right',
      width: 176,
      render: (_: unknown, record: Experiment) => {
        const hideRun =
          !isExperimentFail(record.status) ||
          actionVisibleControl?.retry === false;
        const actions: TableColAction[] = [
          {
            label: (
              <Tooltip content="仅针对执行失败的部分重新评测" theme="dark">
                重试
              </Tooltip>
            ),
            hide: hideRun,
            disabled: retryGuardType === GuardActionType.READONLY,
            onClick: () => handleRetryOnCLick(record),
          },
          {
            label: (
              <Tooltip content="查看详情" theme="dark">
                详情
              </Tooltip>
            ),
            onClick: () => handleDetailOnClick(record),
          },
          {
            label: (
              <Tooltip content="复制实验配置并新建实验" theme="dark">
                复制
              </Tooltip>
            ),
            hide: actionVisibleControl?.copy === false,
            disabled: copyGuardType === GuardActionType.READONLY,
            onClick: () => handleCopyOnClick(record),
          },
        ];
        const isFinalStatus =
          record.status === ExptStatus.Success ||
          record.status === ExptStatus.Failed;

        const exportActionCol = TableExportActionButton
          ? {
              // 自定义情况
              label: (
                <TableExportActionButton
                  onClick={() => {
                    handleExport({
                      record,
                      spaceID,
                      onOpenExportModal,
                      source,
                    });
                  }}
                  disabled={!isFinalStatus}
                />
              ),
              disabled: !isFinalStatus,
            }
          : {
              // 默认
              label: '导出',
              onClick: () => {
                handleExport({
                  record,
                  spaceID,
                  onOpenExportModal,
                  source,
                });
              },
              disabled: !isFinalStatus,
              disabledTooltip: !isFinalStatus
                ? '仅支持导出终态(成功或失败)的实验'
                : undefined,
            };

        // 收起来的操作
        const shrinkActions: TableColAction[] = [
          ...extraShrinkActions,
          exportActionCol,
          {
            label: '导出记录',
            onClick: () => {
              sendEvent(EVENT_NAMES.cozeloop_experiment_export_record_click, {
                from: source,
              });
              handleExportRecord({ record, onOpenExportModal });
            },
          },
          {
            label: '删除',
            type: 'danger',
            hide: actionVisibleControl?.delete === false,
            disabled: deleteGuardType === GuardActionType.READONLY,
            onClick: () => handleDelete({ record, spaceID, onRefresh }),
          },
        ];
        const maxCount = actions.filter(item => !item.hide).length;
        return (
          <TableColActions
            actions={[...actions, ...shrinkActions]}
            maxCount={maxCount}
            textClassName="w-full"
          />
        );
      },
    };
    const idColumn: ColumnProps<Experiment> = {
      title: 'ID',
      disableColumnManage: true,
      dataIndex: 'id',
      key: 'id',
      width: 110,
      render(val: Int64) {
        return <IDRender id={val} useTag={true} />;
      },
    };
    const newColumns: ColumnProps<Experiment>[] = [
      ...(enableIdColumn ? [idColumn] : []),
      ...getExperimentColumns({ spaceID, enableSort }),
    ];

    setColumns([
      ...dealColumnsFromStorage(newColumns, columnManageStorageKey),
      ...(enableActionColumn ? [actionsColumn] : []),
    ]);
    setDefaultColumns([
      ...newColumns,
      ...(enableActionColumn ? [actionsColumn] : []),
    ]);
  }, [
    spaceID,
    copyGuardType,
    retryGuardType,
    deleteGuardType,
    actionVisibleControl,
    isPersonalEnterprise,
    onOpenExportModal,
    handleExport,
    TableExportActionButton,
  ]);

  return {
    columns,
    defaultColumns,
    setColumns,
  };
}
