/* eslint-disable @typescript-eslint/no-explicit-any */
import { useEffect, useRef } from 'react';

interface UseSvgPanZoomParams {
  svgSelector: string;
  viewportSelector: string;
  renderedChart: string;
  zoomStepLength?: number;
}

export const useSvgPanZoom = ({
  svgSelector,
  viewportSelector,
  renderedChart,
  zoomStepLength = 0.25,
}: UseSvgPanZoomParams) => {
  const panZoomTigerRef = useRef<any>(null);
  useEffect(() => {
    if (!renderedChart) {
      return;
    }

    import('svg-pan-zoom').then(svgPanZoom => {
      if (panZoomTigerRef.current) {
        return;
      }
      const el = document.querySelector(svgSelector);
      if (!el) {
        // 选择器未命中元素，优雅降级
        return;
      }
      const panZoomTiger = svgPanZoom.default(svgSelector, {
        viewportSelector,
        mouseWheelZoomEnabled: false,
      });
      panZoomTigerRef.current = panZoomTiger;
    });

    return () => {
      // 清理实例，避免重复初始化与内存泄漏
      if (panZoomTigerRef.current) {
        panZoomTigerRef.current.destroy?.();
        panZoomTigerRef.current = null;
      }
    };
  }, [renderedChart]);

  const zoomIn = () => {
    panZoomTigerRef.current?.zoom(
      panZoomTigerRef.current?.getZoom() + zoomStepLength,
    );
  };

  const zoomOut = () => {
    panZoomTigerRef.current?.zoom(
      panZoomTigerRef.current?.getZoom() - zoomStepLength,
    );
  };

  const fit = () => {
    panZoomTigerRef.current?.fit();
    panZoomTigerRef.current?.center();
  };

  return {
    zoomIn,
    zoomOut,
    fit,
  };
};
