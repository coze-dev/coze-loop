import {
  TraceTrigger,
  useGlobalEvalConfig,
} from '@cozeloop/evaluate-components';
import { Tooltip } from '@coze-arch/coze-design';

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
        <Tooltip theme="dark" content="查看实际输出的trace">
          <div className="flex ml-auto" onClick={e => e.stopPropagation()}>
            <TraceTrigger
              className="ml-1 invisible group-hover:visible"
              traceID={traceID ?? ''}
              platformType={traceEvalTargetPlatformType}
              startTime={startTime}
              endTime={endTime}
            />
          </div>
        </Tooltip>
      ) : null}
    </div>
  );
}
