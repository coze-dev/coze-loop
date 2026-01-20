// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { TypographyText } from '@cozeloop/shared-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { RouteBackAction } from '@cozeloop/base-with-adapter-components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { IconCozSetting } from '@coze-arch/coze-design/icons';
import { Select, Toast } from '@coze-arch/coze-design';

import AddContrastExperiment from './add-contrast-experiment';

export default function ExperimentContrastHeader({
  spaceID,
  experimentCount = 0,
  currentExperiments = [],
  onExperimentIdsChange,
  defaultModuleRoute = 'evaluation/experiments',
}: {
  spaceID: string;
  experimentCount: number;
  currentExperiments: Experiment[];
  defaultModuleRoute?: string;
  onExperimentIdsChange?: (ids: Int64[]) => void;
}) {
  return (
    <header className="flex items-center h-[56px] px-5 gap-2  text-xs">
      <RouteBackAction defaultModuleRoute={defaultModuleRoute} />
      <div className="text-xl font-bold">
        {I18n.t('evaluate_compare_experimentCount_experiments', {
          experimentCount,
        })}
      </div>

      <div className="flex items-center gap-3 ml-auto text-sm">
        <Select
          prefix={I18n.t('benchmark')}
          arrowIcon={<IconCozSetting />}
          placeholder={I18n.t('please_select')}
          style={{ minWidth: 170 }}
          value={currentExperiments?.[0]?.id}
          renderSelectedItem={(item: { name?: React.ReactNode }) => (
            <TypographyText className="!max-w-[200px]">
              {item?.name}
            </TypographyText>
          )}
          optionList={currentExperiments?.map(experiment => ({
            label: (
              <TypographyText className="ml-1 !max-w-[240px]">
                {experiment.name}
              </TypographyText>
            ),

            name: experiment.name,
            value: experiment.id,
          }))}
          onChange={val => {
            let newExperiments = [...currentExperiments];
            const baseExperiment = currentExperiments?.find(
              experiment => experiment.id === val,
            );
            if (baseExperiment) {
              newExperiments = newExperiments.filter(e => e !== baseExperiment);
              newExperiments.unshift(baseExperiment);
            }
            onExperimentIdsChange?.(
              newExperiments?.map(e => e.id ?? '').filter(Boolean),
            );
            Toast.success(I18n.t('benchmark_experiment_switch_success'));
          }}
        />

        <AddContrastExperiment
          currentExperiments={currentExperiments}
          onOk={onExperimentIdsChange}
          onClick={() => {
            sendEvent(EVENT_NAMES.cozeloop_experimen_open_compare_modal, {
              from: 'contrast',
            });
          }}
        />
      </div>
    </header>
  );
}
