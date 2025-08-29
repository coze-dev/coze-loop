import { useState } from 'react';

import { ChipSelect } from '@cozeloop/evaluate-components';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import {
  IconCozInfoCircle,
  IconCozWarningCircleFillPalette,
} from '@coze-arch/coze-design/icons';
import { Banner, Tooltip } from '@coze-arch/coze-design';

import { type ExperimentItem } from '@/types/experiment';
import { FORMAT_LIST } from '@/types';
import { useExperiment } from '@/hooks/use-experiment';
import { ActualOutputWithTrace } from '@/components/experiment';

export default function EvalActualOutputTable({
  item,
  expand,
}: {
  item: ExperimentItem;
  expand?: boolean;
}) {
  const experiment = useExperiment();
  const [format, setFormat] = useState<FieldDisplayFormat>(
    item?.actualOutput?.format || FieldDisplayFormat.Markdown,
  );
  const actualOutput = {
    ...item?.actualOutput,
    format,
  };
  return (
    <div className="text-sm py-3 group">
      <div className="flex items-center justify-between gap-1 mt-2 mb-3 px-5 ">
        <div className="flex gap-1 items-center">
          <div className="font-medium text-xs">actual_output</div>
          <Tooltip theme="dark" content="评测对象的实际输出">
            <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
          </Tooltip>
        </div>
        <ChipSelect
          chipRender="selectedItem"
          value={format}
          size="small"
          className="invisible group-hover:visible"
          optionList={FORMAT_LIST}
          onChange={value => {
            setFormat(value as FieldDisplayFormat);
          }}
        ></ChipSelect>
      </div>
      {item.targetErrorMsg ? (
        <Banner
          type="danger"
          className="rounded-small !px-3 !py-2"
          fullMode={false}
          icon={
            <div className="h-[22px] flex items-center">
              <IconCozWarningCircleFillPalette className="text-[16px] text-[rgb(var(--coze-red-5))]" />
            </div>
          }
          description={item.targetErrorMsg}
        />
      ) : (
        <div className="px-5">
          <ActualOutputWithTrace
            expand={expand}
            content={actualOutput}
            traceID={item?.evalTargetTraceID}
            displayFormat={true}
            startTime={experiment?.start_time}
            endTime={experiment?.end_time}
            className="w-full"
          />
        </div>
      )}
    </div>
  );
}
