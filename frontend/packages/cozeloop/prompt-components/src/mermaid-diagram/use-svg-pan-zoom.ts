// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
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
      const panZoomTiger = svgPanZoom.default(svgSelector, {
        viewportSelector,
        mouseWheelZoomEnabled: false,
      });
      panZoomTigerRef.current = panZoomTiger;
    });
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
