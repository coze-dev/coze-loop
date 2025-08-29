/* eslint-disable @coze-arch/max-line-per-function */
import { Fragment, type ReactNode } from 'react';

import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import {
  type ColumnItem,
  ColumnSelector,
  type Version,
} from '@cozeloop/components';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import { IconCozArrowDown } from '@coze-arch/coze-design/icons';
import { Dropdown, Button, Typography, Divider } from '@coze-arch/coze-design';

import { SubmitVersion } from '../submit-version';
import { useDatasetColumnEdit } from '../dataset-column-edit';
import { useImportItemsModal } from '../../dataset-import-items-modal/use-import-items-modal';
import { useAddItemsPanel } from '../../dataset-add-items-panel/use-add-items-panel';
import { useAddExperiment } from '../../add-experiment/use-add-experiment';
import ReportWrapper from './ReportWrapper';

interface TableHeaderProps {
  datasetDetail?: EvaluationSet;
  columns: ColumnItem[];
  // 批量选择
  batchSelectNode: ReactNode;
  // 版本管理
  versionChangeNode: ReactNode;
  // 数据行展开收起
  datasetItemExpandNode: ReactNode;
  defaultColumnsItems: ColumnItem[];
  setColumns: (columns: ColumnItem[]) => void;
  refreshDatasetDetail: () => void;
  isDraftVersion: boolean;
  currentVersion: Version;
  totalItemCount?: number;
}

export const TableHeader = ({
  datasetDetail,
  columns,
  setColumns,
  batchSelectNode,
  versionChangeNode,
  defaultColumnsItems,
  isDraftVersion,
  currentVersion,
  refreshDatasetDetail,
  datasetItemExpandNode,
  totalItemCount,
}: TableHeaderProps) => {
  //添加行数据
  const { setVisible: setAddItemsVisible, panelNode: addItemsPanelNode } =
    useAddItemsPanel(datasetDetail, refreshDatasetDetail);

  // 导入数据
  const { setVisible: setImportModalVisible, modalNode: importModalNode } =
    useImportItemsModal(datasetDetail, refreshDatasetDetail);
  //编辑列
  const { ColumnEditButton, ColumnEditModal } = useDatasetColumnEdit({
    datasetDetail,
    onRefresh: refreshDatasetDetail,
    totalItemCount,
  });

  //添加实验
  const { ExperimentButton, ExperimentModalNode } = useAddExperiment({
    datasetDetail,
    currentVersion,
    isDraftVersion,
  });
  const ADD_DATA_TYPE_LIST = [
    {
      label: '手动添加',
      onClick: () => {
        setAddItemsVisible(true);
        sendEvent(EVENT_NAMES.cozeloop_dataset_add_data, {
          add_type: 'manual',
        });
      },
    },
    {
      label: '本地导入',
      onClick: () => {
        setImportModalVisible(true);
        sendEvent(EVENT_NAMES.cozeloop_dataset_add_data, {
          add_type: 'file',
        });
      },
    },
  ];
  const setNewColumns = (newColumns: ColumnItem[]) => {
    setColumns(newColumns);
  };

  const headerActionList = [
    {
      key: 'dataset_item_expand',
      triggerNode: datasetItemExpandNode,
    },
    {
      key: 'column_manage',
      triggerNode: (
        <ColumnSelector
          columns={columns}
          onChange={setNewColumns}
          defaultColumns={defaultColumnsItems}
        />
      ),
    },
    {
      key: 'column_edit',
      triggerNode: ColumnEditButton,
      hidden: !isDraftVersion,
      extra: [ColumnEditModal],
    },
    {
      key: 'divider',
      triggerNode: (
        <Divider className="w-[1px] h-[22px] mx-2" layout="vertical" />
      ),
    },
    {
      key: 'add_experiment',
      triggerNode: (
        <ReportWrapper
          reportParams={{
            eventName: EVENT_NAMES.cozeloop_experiement_create,
            params: {
              from: 'datasets',
            },
          }}
        >
          {ExperimentButton}
        </ReportWrapper>
      ),
      extra: [ExperimentModalNode],
    },
    {
      key: 'batch_select',
      triggerNode: batchSelectNode,
      hidden: !isDraftVersion,
    },
    {
      key: 'add_data',
      triggerNode: (
        <Dropdown
          clickToHide
          render={
            <Dropdown.Menu mode="menu">
              {ADD_DATA_TYPE_LIST.map((action, index) => (
                <Dropdown.Item
                  key={index}
                  onClick={() => {
                    setAddItemsVisible(false);
                    action.onClick?.();
                  }}
                  className="min-w-[90px] !p-0 !pl-2"
                >
                  <Typography.Text size="small" className="!text-[13px]">
                    {action.label}
                  </Typography.Text>
                </Dropdown.Item>
              ))}
            </Dropdown.Menu>
          }
        >
          <Button color="primary">
            添加数据
            <IconCozArrowDown className="ml-1" />
          </Button>
        </Dropdown>
      ),
      hidden: !isDraftVersion,
      extra: [addItemsPanelNode, importModalNode],
    },
    {
      key: 'version_manage',
      triggerNode: versionChangeNode,
    },
    {
      key: 'submit_version',
      triggerNode: (
        <SubmitVersion
          datasetDetail={datasetDetail}
          onSubmit={refreshDatasetDetail}
        />
      ),
      hidden: !isDraftVersion,
    },
  ];

  return (
    <div className="flex items-center justify-between">
      <Typography.Text className="!text-fg-plus !text-[16px] !font-medium ">
        数据项
      </Typography.Text>
      <div className="flex items-center justify-end gap-2">
        {headerActionList.map(action =>
          action?.hidden ? null : (
            <Fragment key={action.key}>
              {action.triggerNode}
              {action.extra?.map((extra, index) => (
                <Fragment key={index}>{extra}</Fragment>
              ))}
            </Fragment>
          ),
        )}
      </div>
    </div>
  );
};
