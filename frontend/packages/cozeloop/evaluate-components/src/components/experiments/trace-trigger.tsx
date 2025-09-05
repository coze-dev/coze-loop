import { useState } from 'react';

import classNames from 'classnames';
import { TraceDetailPanel } from '@cozeloop/observation-component-adapter';
import { IconButtonContainer } from '@cozeloop/components';
import { useSpace } from '@cozeloop/biz-hooks-adapter';
import { IconCozNode } from '@coze-arch/coze-design/icons';
import { Tooltip, type TooltipProps } from '@coze-arch/coze-design';

function getTimeString(time: Int64 | undefined) {
  if (!time) {
    return '';
  }
  const timeStr = `${time}`;
  if (timeStr.length === 13) {
    return timeStr;
  }
  if (timeStr.length === 10) {
    return `${timeStr}000`;
  }
}

export function TraceTrigger({
  traceID,
  platformType,
  startTime,
  endTime,
  className,
  tooltipProps,
  ...rest
}: {
  traceID: Int64;
  platformType: string | number;
  startTime?: Int64;
  endTime?: Int64;
  className?: string;
  tooltipProps?: TooltipProps;
}) {
  const [visible, setVisible] = useState(false);
  const { spaceID, space } = useSpace();
  const iconButton = (
    <IconButtonContainer
      {...rest}
      className={classNames('actual-outputy-trace-trigger', className)}
      icon={<IconCozNode />}
      onClick={e => {
        e.stopPropagation();
        setVisible(true);
      }}
    />
  );
  return (
    <>
      {tooltipProps ? (
        <Tooltip {...tooltipProps}>
          <div>{iconButton}</div>
        </Tooltip>
      ) : (
        iconButton
      )}
      {visible ? (
        <TraceDetailPanel
          spaceID={spaceID}
          spaceName={space?.name ?? ''}
          searchType="trace_id"
          platformType={platformType?.toString()}
          id={traceID?.toString()}
          startTime={getTimeString(startTime)}
          endTime={getTimeString(endTime)}
          moduleName="evaluation"
          visible={visible}
          onClose={() => {
            setVisible(false);
          }}
        />
      ) : null}
    </>
  );
}
