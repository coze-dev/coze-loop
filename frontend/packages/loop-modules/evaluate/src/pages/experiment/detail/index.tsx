// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable complexity */
/* eslint-disable @coze-arch/max-line-per-function */
import { useParams, useSearchParams } from 'react-router-dom';
import {
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from 'react';

import classNames from 'classnames';
import { useRequest } from 'ahooks';
import { useEvaluationFlagStore } from '@cozeloop/stores';
import { I18n } from '@cozeloop/i18n-adapter';
import { useBreadcrumb } from '@cozeloop/hooks';
import { useExptTab } from '@cozeloop/evaluate-components';
import { LoopTabs } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { Spin } from '@coze-arch/coze-design';

import {
  batchGetExperiment,
  batchGetExperimentResult,
} from '@/request/experiment';
import { ExperimentContextProvider } from '@/hooks/use-experiment';

import ExperimentHeader from './components/experiment-header';
import ExperimentTable from './components/experiment-detail-table';
import ExperimentDescription from './components/experiment-description';
import ExperimentChart from './components/experiment-chart';

export default function ({
  defaultModuleRoute,
  defaultContrastRoute,
  renderExtraButtons,
}: {
  defaultModuleRoute?: string;
  defaultContrastRoute?: string;
  renderExtraButtons?: (experiment?: Experiment) => ReactNode;
}) {
  const { experimentID = '' } = useParams<{ experimentID: string }>();
  const { spaceID = '' } = useSpace();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeKey, setActiveKey] = useState('detail');
  const [refreshKey, setRefreshKey] = useState('');

  const { getExptTabList } = useExptTab();
  const enableEvaluationAnalysis = useEvaluationFlagStore(
    state => state.enableEvaluationAnalysis,
  );

  const exptTabList = getExptTabList();

  // 从 URL 查询参数初始化 activeKey
  useEffect(() => {
    const tabKeyFromUrl = searchParams.get('tabKey');
    if (tabKeyFromUrl) {
      setActiveKey(tabKeyFromUrl);
    }
  }, [searchParams]);

  // 当 activeKey 变化时更新 URL 查询参数
  const handleActiveKeyChange = useCallback(
    (newActiveKey: string) => {
      setActiveKey(newActiveKey);
      setSearchParams(
        prev => {
          const newParams = new URLSearchParams(prev);
          newParams.set('tabKey', newActiveKey);
          return newParams;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

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

  const tabList = useMemo(() => {
    const result: { tab: React.JSX.Element | string; itemKey: string }[] = [
      { tab: I18n.t('data_detail'), itemKey: 'detail' },
      { tab: I18n.t('measure_stat'), itemKey: 'chart' },
    ];

    if (exptTabList?.length && enableEvaluationAnalysis) {
      result.push(
        ...exptTabList.map(item => ({
          tab: (
            <div className="flex items-center gap-1">
              {item?.name}
              {item?.nameTag}
            </div>
          ),

          itemKey: item.type,
        })),
      );
    }
    return result;
  }, [exptTabList]);

  return (
    <div className="h-full overflow-hidden flex flex-col">
      <ExperimentContextProvider experiment={base.data?.experiment}>
        <ExperimentHeader
          experiment={base.data?.experiment}
          spaceID={spaceID}
          onRefreshExperiment={base.refresh}
          onRefresh={onRefresh}
          defaultModuleRoute={defaultModuleRoute}
          defaultContrastRoute={defaultContrastRoute}
          renderExtraButtons={renderExtraButtons}
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
          onChange={handleActiveKeyChange}
          tabPaneMotion={false}
          keepDOM={false}
          tabList={tabList}
        />

        <div className="grow overflow-hidden pb-5">
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
          {exptTabList?.length && enableEvaluationAnalysis
            ? exptTabList.map(item => {
                const Component = item?.tabComponent;
                if (!Component && activeKey !== item.type) {
                  return null;
                }
                return (
                  <div key={item.type} className="h-full">
                    {Component ? (
                      <Component
                        experiment={base.data?.experiment as Experiment}
                      />
                    ) : null}
                  </div>
                );
              })
            : null}
        </div>
      </ExperimentContextProvider>
    </div>
  );
}
