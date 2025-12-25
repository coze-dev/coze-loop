// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import classNames from 'classnames';
import { observationTraceAdapters } from '@cozeloop/observation-adapter';
import { IconButtonContainer } from '@cozeloop/components';
import { IconCozNode } from '@coze-arch/coze-design/icons';
import { Button, Tooltip, type TooltipProps } from '@coze-arch/coze-design';

const { TraceDetailPanel } = observationTraceAdapters;

function getTimeString(time: Int64 | undefined) {
  if (!time) {
    return undefined;
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
  content,
  ...rest
}: {
  traceID: Int64;
  platformType: string | number;
  startTime?: Int64;
  endTime?: Int64;
  className?: string;
  tooltipProps?: TooltipProps;
  content?: React.ReactNode;
}) {
  const [visible, setVisible] = useState(false);
  const iconButton = (
    <IconButtonContainer
      {...rest}
      onClick={e => {
        e.stopPropagation();
        setVisible(true);
      }}
      className={classNames('actual-outputy-trace-trigger', className)}
      icon={<IconCozNode />}
    />
  );
  return (
    <>
      {tooltipProps ? (
        <Tooltip {...tooltipProps}>
          <div>
            {content ? (
              <Button
                onClick={e => {
                  e.stopPropagation();
                  setVisible(true);
                }}
                size="mini"
                color="secondary"
                icon={<IconCozNode />}
              >
                {content}
              </Button>
            ) : (
              iconButton
            )}
          </div>
        </Tooltip>
      ) : (
        iconButton
      )}
      {visible ? (
        <TraceDetailPanel
          platformType={platformType?.toString()}
          traceID={traceID?.toString()}
          startTime={getTimeString(startTime)}
          endTime={getTimeString(endTime)}
          visible={visible}
          onClose={() => {
            setVisible(false);
          }}
        />
      ) : null}
    </>
  );
}
