// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { useDebounceFn } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import { ExperimentExportListEmptyState } from '@cozeloop/evaluate-components';
import { TableWithPagination } from '@cozeloop/components';
import {
  type ExptResultExportRecord,
  type Experiment,
} from '@cozeloop/api-schema/evaluation';
import {
  IconCozInfoCircleFill,
  IconCozRefresh,
} from '@coze-arch/coze-design/icons';
import { Banner, Button, Modal } from '@coze-arch/coze-design';

import { useExperimentExportData } from './use-experiment-export-data';
import { useExportExperimentListColumns } from './use-experiment-export-columns';

import styles from './index.module.less';

interface ExportTableModalProps {
  visible: boolean;
  setVisible: (visible: boolean) => void;
  experiment?: Experiment;
  source?: string;
}

const ExportRefreshComp = ({ onRefresh }: { onRefresh: () => void }) => (
  <div className="flex items-center gap-[6px]">
    <span>{I18n.t('evaluate_export_records')}</span>
    <Button
      color="secondary"
      iconPosition="left"
      size="small"
      icon={<IconCozRefresh color={'var(--coz-fg-secondary)'} />}
      onClick={() => {
        onRefresh();
      }}
    >
      <span className="text-[var(--coz-fg-secondary)]">
        {I18n.t('refresh')}
      </span>
    </Button>
  </div>
);

const ExportTableModal = (props: ExportTableModalProps) => {
  const { visible, setVisible, experiment, source } = props;
  const [modalLoading, setModalLoading] = useState(false);
  const { service } = useExperimentExportData(experiment?.id ?? '');
  const { columns } = useExportExperimentListColumns({
    columnManageStorageKey: 'export_table_column_manage',
    setModalLoading,
    source,
  });

  const { run: onRefreshDebounce } = useDebounceFn(
    () => {
      service.run({
        current: service.pagination?.current ?? 1,
        pageSize: service.pagination?.pageSize,
      });
    },
    { wait: 500 },
  );

  const onTableRefresh = async () => {
    setModalLoading(true);
    await onRefreshDebounce();
    setModalLoading(false);
  };

  const table = (
    <TableWithPagination<ExptResultExportRecord>
      style={{ minHeight: '480px', height: '480px' }}
      service={service}
      heightFull={true}
      pageSizeStorageKey="export_table_page_size"
      showTableWhenEmpty={true}
      tableProps={{
        rowKey: 'export_id',
        columns,
        sticky: { top: 0 },
        loading: service.loading || modalLoading,
      }}
      empty={<ExperimentExportListEmptyState />}
    />
  );

  useEffect(() => {
    if (visible) {
      onTableRefresh();
    }
  }, [visible]);

  return (
    <Modal
      type="modal"
      width={'70%'}
      className={styles['export-record-table-wrapper']}
      footer={null}
      visible={visible}
      title={<ExportRefreshComp onRefresh={onTableRefresh} />}
      onOk={() => setVisible(false)}
      onCancel={() => setVisible(false)}
    >
      <Banner
        className="mb-3 rounded-small"
        justify="start"
        fullMode={true}
        description={
          <div className="flex items-center">
            <IconCozInfoCircleFill className="mr-1.5 text-[#5A4DED]" />
            <div>{I18n.t('export_files_save_limit_100_days')}</div>
          </div>
        }
      />
      <div className="mh-[500px]">{table}</div>
    </Modal>
  );
};

export default ExportTableModal;
