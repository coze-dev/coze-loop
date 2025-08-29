import { useParams } from 'react-router-dom';
import { useCallback, useState } from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { LoopTabs } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import { Layout, Spin } from '@coze-arch/coze-design';

import {
  batchGetExperiment,
  batchGetExperimentResult,
} from '@/request/experiment';
import { ExperimentContextProvider } from '@/hooks/use-experiment';

import ExperimentHeader from './components/experiment-header';
import ExperimentTable from './components/experiment-detail-table';
import ExperimentDescription from './components/experiment-description';
import ExperimentChart from './components/experiment-chart';

export default function () {
  const { experimentID = '' } = useParams<{ experimentID: string }>();
  const { spaceID = '' } = useSpace();
  const [activeKey, setActiveKey] = useState('detail');
  const [refreshKey, setRefreshKey] = useState('');

  const base = useRequest(
    async () => {
      if (!experimentID) {
        return;
      }

      const [exp, expResult] = await Promise.all([
        batchGetExperiment({
          workspace_id: spaceID,
          expt_ids: [experimentID],
        }),
        batchGetExperimentResult({
          workspace_id: spaceID,
          baseline_experiment_id: experimentID,
          experiment_ids: [experimentID],
          page_number: 1,
          page_size: 1,
          use_accelerator: true,
        }),
      ]);

      return {
        experiment: exp.experiments?.[0],

        columnEvaluators:
          (expResult.expt_column_evaluators || []).filter(
            item => item.experiment_id === experimentID,
          )[0]?.column_evaluators ?? [],

        columnAnnotations:
          (expResult.expt_column_annotations ?? []).filter(
            item => item.experiment_id === experimentID,
          )[0]?.column_annotations ?? [],
      };
    },
    {
      refreshDeps: [experimentID, refreshKey],
    },
  );

  useBreadcrumb({
    text: base.data?.experiment?.name || '',
  });

  const onRefresh = useCallback(() => {
    setRefreshKey(Date.now().toString());
  }, [setRefreshKey]);

  return (
    <Layout className="h-full overflow-hidden flex flex-col">
      <ExperimentContextProvider experiment={base.data?.experiment}>
        <ExperimentHeader
          experiment={base.data?.experiment}
          spaceID={spaceID}
          onRefreshExperiment={base.refresh}
          onRefresh={onRefresh}
        />
        <Spin spinning={base.loading}>
          <div className="px-6 pt-3 pb-6 flex items-center text-sm">
            <ExperimentDescription
              experiment={base.data?.experiment}
              spaceID={spaceID}
            />
          </div>
        </Spin>
        <LoopTabs
          type="card"
          activeKey={activeKey}
          onChange={setActiveKey}
          tabPaneMotion={false}
          keepDOM={false}
          tabList={[
            { tab: '数据明细', itemKey: 'detail' },
            { tab: '指标统计', itemKey: 'chart' },
          ]}
        />
        <div className="grow overflow-hidden">
          <div
            className={classNames(
              'h-full overflow-hidden px-6 pt-4 pb-4',
              activeKey === 'detail' ? '' : 'hidden',
            )}
          >
            <ExperimentTable
              spaceID={spaceID}
              experimentID={experimentID}
              refreshKey={refreshKey}
              experiment={base.data?.experiment}
              onRefreshPage={onRefresh}
            />
          </div>
          {activeKey === 'chart' && (
            <div className="h-full overflow-auto styled-scrollbar pl-6 pr-[18px] py-4">
              <ExperimentChart
                spaceID={spaceID}
                experiment={base.data?.experiment}
                columnEvaluators={base.data?.columnEvaluators}
                columnAnnotations={base.data?.columnAnnotations}
                experimentID={experimentID}
                loading={base.loading}
              />
            </div>
          )}
        </div>
      </ExperimentContextProvider>
    </Layout>
  );
}
