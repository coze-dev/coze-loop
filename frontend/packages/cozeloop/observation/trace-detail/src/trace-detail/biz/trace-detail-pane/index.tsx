// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useRef, useState } from 'react';

import { clamp } from 'lodash-es';
import classNames from 'classnames';
import { useMouseDownOffset } from '@cozeloop/hooks';
import { Layout, SideSheet } from '@coze-arch/coze-design';

import { type TraceDetailContext } from '@/trace-detail';
import { PERCENT } from '@/consts';

import { type TraceDetailProps } from '../trace-detail/interface';
import { TraceDetail } from '../trace-detail';
import { DEFAULT_WIDTH, MAX_WIDTH, MIN_WIDTH } from './config';

export interface TraceDetailPanelProps
  extends Omit<TraceDetailProps, 'layout' | 'spanDetailConfig'> {
  visible: boolean;
  onClose: () => void;
}

export const TraceDetailPanel = ({
  visible,
  onClose,
  headerConfig,
  ...props
}: TraceDetailPanelProps & TraceDetailContext) => {
  const [sidePaneWidth, setSidePaneWidth] = useState(DEFAULT_WIDTH);
  const prevWidthRef = useRef(sidePaneWidth);

  const { ref, isActive } = useMouseDownOffset(({ offsetX }) => {
    const newWidth =
      prevWidthRef.current - (offsetX / document.body.clientWidth) * PERCENT;
    setSidePaneWidth(clamp(newWidth, MIN_WIDTH, MAX_WIDTH));
  });

  useEffect(() => {
    prevWidthRef.current = sidePaneWidth;
    document.body.style.cursor = isActive ? 'col-resize' : '';
    document.body.style.userSelect = isActive ? 'none' : 'auto';
  }, [isActive, sidePaneWidth]);

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      closeIcon={null}
      width={`${sidePaneWidth}%`}
      headerStyle={{ display: 'none' }}
      bodyStyle={{
        padding: 0,
      }}
    >
      <div
        ref={ref}
        className={classNames(
          'absolute h-full w-[3px] bg-transparent z-50 top-0 left-0 hover:cursor-col-resize hover:bg-[rgb(var(--coze-up-brand-7))] transition',
          {
            'bg-[rgb(var(--coze-up-brand-7))] cursor-col-resize': isActive,
          },
        )}
      />
      <div id="trace-detail-side-sheet-panel" className="relative h-full">
        <Layout.Content className="h-full flex flex-col !m-0 pb-0 overflow-hidden">
          <TraceDetail
            layout="horizontal"
            headerConfig={{
              showClose: true,
              onClose,
              minColWidth: 180,
              ...headerConfig,
            }}
            spanDetailConfig={{
              baseInfoPosition: 'right',
            }}
            {...props}
          />
        </Layout.Content>
      </div>
    </SideSheet>
  );
};
