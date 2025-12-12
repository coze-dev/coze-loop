import { useEffect, useRef, useState } from 'react';

import { clamp } from 'lodash-es';
import cls from 'classnames';
import { Layout, SideSheet } from '@coze-arch/coze-design';

import { useMouseDownOffset } from '@/shared/hooks/use-mouse-down-offset';
import { PERCENT } from '@/shared/constants';
import { type TraceDetailContext } from '@/features/trace-detail/hooks/use-trace-detail-context';
import { type CozeloopTraceDetailProps } from '@/features/trace-detail/containers/trace-detail/interface';
import { CozeloopTraceDetail } from '@/features/trace-detail/containers/trace-detail';

import { DEFAULT_WIDTH, MAX_WIDTH, MIN_WIDTH } from './config';

export interface CozeloopTraceDetailPanelProps
  extends Omit<CozeloopTraceDetailProps, 'layout'> {
  visible: boolean;
  onClose: () => void;
}

export const CozeloopTraceDetailPanel = ({
  visible,
  onClose,
  ...props
}: CozeloopTraceDetailPanelProps & TraceDetailContext) => {
  const [sidePaneWidth, setSidePaneWidth] = useState(DEFAULT_WIDTH);
  const [isFullscreen, setIsFullscreen] = useState(false);
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

  const handleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
  };

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      closeIcon={null}
      width={isFullscreen ? '100vw' : `${sidePaneWidth}%`}
      headerStyle={{ display: 'none' }}
      bodyStyle={{
        padding: 0,
      }}
    >
      <div
        ref={ref}
        className={cls(
          'absolute h-full w-[3px] bg-transparent z-50 top-0 left-0 hover:cursor-col-resize hover:bg-[rgb(var(--coze-up-brand-7))] transition',
          {
            'bg-[rgb(var(--coze-up-brand-7))] cursor-col-resize': isActive,
          },
        )}
      />
      <div id="trace-detail-side-sheet-panel" className="relative h-full">
        <Layout.Content className="h-full flex flex-col !m-0 pb-0 overflow-hidden">
          <CozeloopTraceDetail
            {...props}
            layout="horizontal"
            headerConfig={{
              showClose: true,
              onClose,
              minColWidth: 180,
              showFullscreenButton: true,
              onFullscreen: handleFullscreen,
              ...props.headerConfig,
            }}
            spanDetailConfig={{
              baseInfoPosition: 'right',
              ...props.spanDetailConfig,
            }}
          />
        </Layout.Content>
      </div>
    </SideSheet>
  );
};
