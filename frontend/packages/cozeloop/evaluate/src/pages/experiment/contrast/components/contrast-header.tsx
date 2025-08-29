import { EVENT_NAMES, sendEvent } from '@cozeloop/tea-adapter';
import { TypographyText } from '@cozeloop/evaluate-components';
import { RouteBackAction } from '@cozeloop/components';
import { type Experiment } from '@cozeloop/api-schema/evaluation';
import { IconCozSetting } from '@coze-arch/coze-design/icons';
import { Select, Toast } from '@coze-arch/coze-design';

import AddContrastExperiment from './add-contrast-experiment';

export default function ExperimentContrastHeader({
  spaceID,
  experimentCount = 0,
  currentExperiments = [],
  onExperimentIdsChange,
}: {
  spaceID: string;
  experimentCount: number;
  currentExperiments: Experiment[];
  onExperimentIdsChange?: (ids: Int64[]) => void;
}) {
  return (
    <header className="flex items-center h-[56px] px-5 gap-2  text-xs">
      <RouteBackAction defaultModuleRoute="evaluation/experiments" />
      <div className="text-xl font-bold">对比{experimentCount}个实验</div>

      <div className="flex items-center gap-3 ml-auto text-sm">
        <Select
          prefix="基准"
          arrowIcon={<IconCozSetting />}
          placeholder="请选择"
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
            Toast.success('基准实验切换成功');
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
