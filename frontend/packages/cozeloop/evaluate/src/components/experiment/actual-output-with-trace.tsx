// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import {
  TraceTrigger,
  useGlobalEvalConfig,
} from '@cozeloop/evaluate-components';

import { CellContentRender } from '@/utils/experiment';
import { type DatasetCellContent } from '@/types';

/** 渲染实际输出，可hover查看trace */
export default function ActualOutputWithTrace({
  expand,
  content,
  traceID,
  startTime,
  endTime,
  enableTrace = true,
  displayFormat = false,
  className,
}: {
  content: DatasetCellContent | undefined;
  traceID: Int64 | undefined;
  startTime?: Int64;
  endTime?: Int64;
  expand?: boolean;
  enableTrace?: boolean;
  displayFormat?: boolean;
  className?: string;
}) {
  const { traceEvalTargetPlatformType } = useGlobalEvalConfig();

  return (
    <div
      className="group flex leading-5 w-full min-h-[20px] overflow-hidden"
      onClick={e => e.stopPropagation()}
      // 选中文本时不触发阻止冒泡
      // onClick={e => {
      //   const selection = window.getSelection();
      //   if (!selection?.isCollapsed) {
      //     e.stopPropagation();
      //   }
      // }}
    >
      <CellContentRender
        expand={expand}
        content={content}
        displayFormat={displayFormat}
        className={className}
      />

      {enableTrace && traceID ? (
        <div className="flex ml-auto" onClick={e => e.stopPropagation()}>
          <TraceTrigger
            className="ml-1 invisible group-hover:visible"
            traceID={traceID ?? ''}
            platformType={traceEvalTargetPlatformType}
            startTime={startTime}
            endTime={endTime}
            tooltipProps={{
              content: I18n.t('view_actual_output_trace'),
              theme: 'dark',
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
