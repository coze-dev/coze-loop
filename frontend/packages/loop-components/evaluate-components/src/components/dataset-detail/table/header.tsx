// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/max-line-per-function */
import { Fragment, type ReactNode, useEffect, useState } from 'react';

import { useDebounceFn } from 'ahooks';
import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { EvaluationAddDataDropdownMenus } from '@cozeloop/evaluate-adapter/add-data-dropdown';
import {
  type ColumnItem,
  ColumnSelector,
  type Version,
} from '@cozeloop/components';
import { type EvaluationSet } from '@cozeloop/api-schema/evaluation';
import {
  IconCozArrowDown,
  IconCozImport,
  IconCozMagnifier,
  IconCozPlus,
} from '@coze-arch/coze-design/icons';
import {
  Dropdown,
  Button,
  Typography,
  Divider,
  Search,
} from '@coze-arch/coze-design';

import { SubmitVersion } from '../submit-version';
import { useDatasetColumnEdit } from '../dataset-column-edit';
import { useImportItemsModal } from '../../dataset-import-items-modal/use-import-items-modal';
import { useAddItemsPanel } from '../../dataset-add-items-panel/use-add-items-panel';
import { useAddExperiment } from '../../add-experiment/use-add-experiment';
import ReportWrapper from './ReportWrapper';

const DATASET_TAGS_SEARCH_MAX_LENGTH = 100;
const DATASET_TAGS_SEARCH_DEBOUNCE_TIME = 500;

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
  tagsSearchValue?: string;
  onTagsSearchValueChange?: (value: string) => void;
}

interface DatasetTagsSearchProps {
  value?: string;
  onChange?: (value: string) => void;
}

const DatasetTagsSearch = ({ value, onChange }: DatasetTagsSearchProps) => {
  const [innerValue, setInnerValue] = useState(value ?? '');
  const { run: runSearch } = useDebounceFn(
    (nextValue: string) => {
      onChange?.(nextValue.trim());
    },
    {
      wait: DATASET_TAGS_SEARCH_DEBOUNCE_TIME,
    },
  );

  useEffect(() => {
    setInnerValue(value ?? '');
  }, [value]);

  const handleChange = (nextValue?: string) => {
    const normalizedValue = (nextValue ?? '').slice(
      0,
      DATASET_TAGS_SEARCH_MAX_LENGTH,
    );
    setInnerValue(normalizedValue);
    runSearch(normalizedValue);
  };

  return (
    <div className="w-60 shrink-0">
      <Search
        className="!w-full"
        placeholder="搜索 tags"
        value={innerValue}
        onChange={handleChange}
        prefix={<IconCozMagnifier />}
        showClear
        autoComplete="off"
      />
    </div>
  );
};

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
  tagsSearchValue,
  onTagsSearchValueChange,
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
      label: I18n.t('add_manually'),
      icon: <IconCozPlus />,
      onClick: () => {
        setAddItemsVisible(true);
        sendEvent(EVENT_NAMES.cozeloop_dataset_add_data, {
          add_type: 'manual',
        });
      },
    },
    {
      label: I18n.t('local_import'),
      icon: <IconCozImport />,
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
            <EvaluationAddDataDropdownMenus
              evaluationSet={datasetDetail}
              menuConfigs={ADD_DATA_TYPE_LIST}
            />
          }
        >
          <Button color="primary">
            {I18n.t('add_data')}
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
    <div className="flex items-center justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <Typography.Text className="shrink-0 !text-fg-plus !text-[16px] !font-medium ">
          {I18n.t('data_item')}
        </Typography.Text>
        <DatasetTagsSearch
          value={tagsSearchValue}
          onChange={onTagsSearchValueChange}
        />
      </div>
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
