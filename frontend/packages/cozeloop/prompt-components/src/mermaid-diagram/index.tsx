/* eslint-disable @coze-arch/use-error-in-catch */
import {
  useState,
  useEffect,
  useRef,
  forwardRef,
  useImperativeHandle,
} from 'react';

import mermaid from 'mermaid';
import { isEmpty, uniqueId } from 'lodash-es';
import cls from 'classnames';

import { exportImage } from './utils';
import { useSvgPanZoom } from './use-svg-pan-zoom';

import styles from './index.module.less';

export interface MermaidDiagramProps {
  /** mermaid text */
  chart: string;
  className?: string;
}
export interface MermaidDiagramRef {
  zoomIn: () => void;
  zoomOut: () => void;
  fit: () => void;
  exportImg: () => Promise<void>;
}

const DEFAULT_THEME = {
  theme: 'base',
  themeVariables: {
    background: '#f9fafb',
    fontSize: '12px',
    primaryColor: '#E5F6FF',
    primaryTextColor: 'rgba(0,0,0,0.8)',
    secondaryColor: '#F5EBFF',
    primaryBorderColor: '#73A6FF',
    secondaryBorderColor: '#BE8FED',
    secondaryTextColor: 'rgba(0,0,0,0.56)',
    tertiaryColor: '#FFF6CC',
    tertiaryBorderColor: '#FFBC52',
    tertiaryTextColor: 'rgba(0,0,0,0.3)',
    noteBkgColor: '#F3F4F6',
    noteTextColor: 'rgba(0,0,0,0.55)',
    noteBorderColor: '#A8A8A8',
    lineColor: 'rgba(0,0,0,0.3)',
    textColor: 'rgba(0,0,0,0.8)',
    errorBkgColor: '#FFEBEA',
    errorTextColor: '#FF3B30',
  },
} as const;

export const MermaidDiagram = /*#__PURE__*/ forwardRef<
  MermaidDiagramRef,
  MermaidDiagramProps
>(function MermaidDiagram({ chart, className }, outerRef) {
  const ref = useRef<HTMLDivElement>(null);
  const id = useRef(`mermaid-diagram-${uniqueId()}`);
  const svgId = `svg-${id.current}`;
  const [preChart, setPreChart] = useState('');

  const updateChart = async (chartStr: string) => {
    if (isEmpty(chartStr)) {
      return;
    }
    try {
      const isValidChart = await mermaid.parse(chartStr, {
        suppressErrors: true,
      });
      if (!isValidChart) {
        return;
      }
      const renderResult = await mermaid.render(
        svgId,
        chartStr,
        ref.current || undefined,
      );
      if (ref.current && renderResult.svg) {
        ref.current.innerHTML = renderResult.svg;
        renderResult.bindFunctions?.(ref.current);

        setPreChart(chartStr);
      }
    } catch (e) {
      if (chartStr !== preChart) {
        /**
         * 出错情况下，尝试渲染上次成功的图表
         * ps: gantt图存在parse未检测出的错误导致错误抛出的情况
         */
        updateChart(preChart);
      }
    }
  };
  useEffect(() => {
    mermaid.initialize({
      ...DEFAULT_THEME,
      startOnLoad: false,
      logLevel: 'error',
      suppressErrorRendering: true,
    });
  }, []);

  useEffect(() => {
    updateChart(chart);
  }, [chart]);

  const { zoomIn, zoomOut, fit } = useSvgPanZoom({
    svgSelector: `#${svgId}`,
    viewportSelector: id.current,
    renderedChart: preChart,
  });

  useImperativeHandle(outerRef, () => ({
    zoomIn,
    zoomOut,
    fit,
    exportImg: async () => exportImage(`#${svgId}`),
  }));

  return (
    <div
      id={id.current}
      className={cls(
        'w-full min-w-[330px] h-[300px]',
        styles.container,
        className,
      )}
      ref={ref}
    ></div>
  );
});
