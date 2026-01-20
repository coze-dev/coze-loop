// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useRef, useState } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import {
  ChipSelect,
  TraceTrigger,
  useGlobalEvalConfig,
} from '@cozeloop/evaluate-components';
import { FieldDisplayFormat } from '@cozeloop/api-schema/data';
import {
  IconCozInfoCircle,
  IconCozWarningCircleFillPalette,
} from '@coze-arch/coze-design/icons';
import { Banner, Tooltip, Typography } from '@coze-arch/coze-design';

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
  const { traceEvalTargetPlatformType } = useGlobalEvalConfig();
  const [format, setFormat] = useState<FieldDisplayFormat>(
    item?.actualOutput?.format || FieldDisplayFormat.Markdown,
  );
  const ref = useRef<HTMLDivElement>(null);
  const actualOutput = {
    ...item?.actualOutput,
    format,
  };
  return (
    <div className="text-sm py-3 group relative" ref={ref}>
      <div className="flex items-center justify-between gap-1 mt-2 mb-3 px-5 ">
        <div className="flex gap-1 items-center">
          <div className="font-medium text-xs">actual_output</div>
          <Tooltip
            theme="dark"
            content={I18n.t('evaluation_object_actual_output')}
          >
            <IconCozInfoCircle className="text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]" />
          </Tooltip>
        </div>
        <div className="flex gap-1 items-center">
          <ChipSelect
            chipRender="selectedItem"
            value={format}
            size="small"
            getPopupContainer={() => ref.current || document.body}
            className="invisible group-hover:visible"
            optionList={FORMAT_LIST}
            onChange={value => {
              setFormat(value as FieldDisplayFormat);
            }}
          ></ChipSelect>
          {item?.evalTargetTraceID ? (
            <div
              className="flex ml-auto shrink-0"
              onClick={e => e.stopPropagation()}
            >
              <TraceTrigger
                className="ml-1 group-hover:visible"
                traceID={item?.evalTargetTraceID ?? ''}
                platformType={traceEvalTargetPlatformType}
                startTime={experiment?.start_time}
                endTime={experiment?.end_time}
                content={
                  <Typography.Text className="text-[12px] text-[var(--coz-fg-secondary)] hover:text-[var(--coz-fg-primary)]">
                    {I18n.t('evaluate_view_trace')}
                  </Typography.Text>
                }
                tooltipProps={{
                  content: I18n.t('evaluate_view_actual_output_trace'),
                  theme: 'dark',
                }}
              />
            </div>
          ) : null}
        </div>
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
            enableTrace={false}
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
