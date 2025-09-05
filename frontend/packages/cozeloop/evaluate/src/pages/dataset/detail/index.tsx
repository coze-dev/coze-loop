// import { useParams } from 'react-router-dom';
import { useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  useFetchDatasetDetail,
  DatasetItemList,
  DatasetDetailHeader,
  DatasetVersionTag,
  DatasetRelatedExperiment,
} from '@cozeloop/evaluate-components';
import { LoopTabs } from '@cozeloop/components';
import { type Version } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { Layout, Loading, Tabs } from '@coze-arch/coze-design';

import ExportTableModal from '@/components/experiment/experiment-export/export-table-modal';

enum TabKey {
  EVAL = 'eval',
  EXPERIMENT = 'experiment',
}

export default function EvaluateSetDetailPage() {
  const { spaceID } = useSpace();
  const { datasetDetail, refreshDataset, loading } = useFetchDatasetDetail();
  const [version, setCurrentVersion] = useState<Version>();
  const [activeTab, setActiveTab] = useState<TabKey>(TabKey.EVAL);

  // 导出记录弹窗状态
  const [exportModalVisible, setExportModalVisible] = useState(false);
  const [selectedExperiment, setSelectedExperiment] = useState<Experiment>();

  // 处理导出记录弹窗打开
  const handleOpenExportModal = (experiment: Experiment) => {
    setSelectedExperiment(experiment);
    setExportModalVisible(true);
  };
  useBreadcrumb({
    text: datasetDetail?.name || '',
  });
  return (
    <Layout.Content className="w-full h-full overflow-hidden flex flex-col items-center justify-center !px-0">
      {loading ? (
        <Loading loading={true} />
      ) : (
        <>
          <DatasetDetailHeader
            datasetDetail={datasetDetail}
            onRefresh={() => {
              refreshDataset();
            }}
          />
          <LoopTabs
            className="flex-1 mt-4 overflow-hidden w-full"
            type="card"
            activeKey={activeTab}
            lazyRender={true}
            onChange={key => setActiveTab(key as TabKey)}
          >
            <Tabs.TabPane
              itemKey={TabKey.EVAL}
              tab={
                <>
                  <span className="mr-2">{I18n.t('evaluation_set')}</span>
                  <DatasetVersionTag
                    currentVersion={version}
                    datasetDetail={datasetDetail}
                  />
                </>
              }
            >
              {datasetDetail ? (
                <DatasetItemList
                  setCurrentVersion={setCurrentVersion}
                  datasetDetail={datasetDetail}
                  spaceID={spaceID}
                  refreshDatasetDetail={refreshDataset}
                />
              ) : null}
            </Tabs.TabPane>
            <Tabs.TabPane
              itemKey={TabKey.EXPERIMENT}
              tab={I18n.t('associated_experiment')}
            >
              <DatasetRelatedExperiment
                spaceID={spaceID}
                datasetID={datasetDetail?.id ?? ''}
                className="pl-6 pr-[18px] h-full overflow-auto styled-scrollbar"
                sourceName="related_dataset"
                sourcePath={`evaluation/datasets/${datasetDetail?.id}`}
                experimentsColumnsOptions={{
                  onOpenExportModal: handleOpenExportModal,
                }}
              />
            </Tabs.TabPane>
          </LoopTabs>

          {/* 导出记录弹窗 */}
          <ExportTableModal
            visible={exportModalVisible}
            setVisible={setExportModalVisible}
            experiment={selectedExperiment}
            source="related_dataset"
          />
        </>
      )}
    </Layout.Content>
  );
}
