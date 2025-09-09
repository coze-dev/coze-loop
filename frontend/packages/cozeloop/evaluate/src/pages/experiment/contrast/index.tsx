// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSearchParams } from 'react-router-dom';
import { useEffect, useState } from 'react';

import { isEqual } from 'lodash-es';
import { useRequest } from 'ahooks';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  verifyContrastExperiment,
  ExperimentContrastChart,
} from '@cozeloop/evaluate-components';
import { LoopTabs } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { useBreadcrumb } from '@cozeloop/base-hooks';
import { type Experiment } from '@cozeloop/api-schema/evaluation';

import { batchGetExperiment } from '@/request/experiment';

import ExperimentContrastTable from './components/contrast-table';
import ExperimentContrastHeader from './components/contrast-header';

export default function ExperimentContrast() {
  const { spaceID } = useSpace();
  const [experimentIds, setExperimentIds] = useState<string[]>([]);
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [activeKey, setActiveKey] = useState('detail');
  const [searchParams, setSearchParams] = useSearchParams();

  useBreadcrumb({
    text: `${I18n.t('compare_placeholder1_experiments', { placeholder1: experimentIds.length })}`,
  });

  const service = useRequest(
    async () => {
      if (experimentIds.length === 0) {
        return { total: 0, list: [] };
      }
      const res = await batchGetExperiment({
        workspace_id: spaceID,
        expt_ids: experimentIds || [],
      });
      return {
        total: Number(res.experiments?.length) || 0,
        list: res.experiments ?? [],
      };
    },
    { refreshDeps: [experimentIds] },
  );

  const handleRefresh = () => service.refresh();

  useEffect(() => {
    const newExperiments = service.data?.list ?? [];
    if (!verifyContrastExperiment(newExperiments)) {
      return;
    }
    setExperiments(newExperiments);
  }, [service.data?.list]);

  useEffect(() => {
    const experimentIdsFromUrl =
      searchParams.get('experiment_ids')?.split(',') ?? [];

    setExperimentIds(originIds => {
      if (!isEqual(experimentIdsFromUrl, originIds)) {
        return experimentIdsFromUrl;
      }
      return originIds;
    });
  }, [searchParams]);

  return (
    <div className="h-full overflow-hidden flex flex-col gap-[16px]">
      <div className="flex flex-col gap-[6px]">
        <ExperimentContrastHeader
          spaceID={spaceID}
          currentExperiments={experiments}
          experimentCount={experimentIds.length}
          onExperimentIdsChange={ids => {
            setSearchParams({ experiment_ids: ids.join(',') });
          }}
        />
        <LoopTabs
          type="card"
          activeKey={activeKey}
          onChange={setActiveKey}
          tabPaneMotion={false}
          keepDOM={false}
          tabList={[
            { tab: I18n.t('data_detail'), itemKey: 'detail' },
            { tab: I18n.t('measure_stat'), itemKey: 'chart' },
          ]}
        />
      </div>
      <div className="grow overflow-hidden">
        {activeKey === 'detail' && (
          <div className="h-full overflow-hidden px-6 pb-4">
            <ExperimentContrastTable
              spaceID={spaceID}
              experiments={experiments}
              experimentIds={experimentIds}
              onExperimentChange={exprs => {
                const ids = exprs.map(item => item.id ?? '');
                setSearchParams({ experiment_ids: ids.join(',') });
              }}
            />
          </div>
        )}
        {activeKey === 'chart' && (
          <div className="h-full overflow-auto styled-scrollbar pl-6 pr-[18px] pb-4">
            <ExperimentContrastChart
              spaceID={spaceID}
              loading={service.loading}
              experiments={experiments}
              experimentIds={experimentIds}
              onRefresh={handleRefresh}
            />
          </div>
        )}
      </div>
    </div>
  );
}
