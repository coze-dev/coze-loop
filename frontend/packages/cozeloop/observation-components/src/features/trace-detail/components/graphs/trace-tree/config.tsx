// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type LineStyle } from '../tree/typing';
import { type TraceTreeProps } from './type';

type DefaultProps = Pick<TraceTreeProps, 'lineStyle' | 'globalStyle'>;

export const defaultProps: DefaultProps = {
  lineStyle: {
    normal: {
      stroke: '#D0D4E0',
      strokeWidth: 1,
    },
    hover: {
      stroke: 'var(--semi-color-primary-hover)',
      strokeWidth: 2,
    },
    select: {
      stroke: '#5A4DED',
      strokeWidth: 1,
    },
    error: {
      stroke: '#D0292F',
    },
  },
  globalStyle: {
    nodeBoxHeight: 60,
    verticalInterval: 8,
    offsetX: 10,
  },
};

interface SpanStatusConfig {
  [spanStatus: string]: {
    lineStyle?: LineStyle;
  };
}

export const spanStatusConfig: SpanStatusConfig = {
  success: {},
  error: {
    lineStyle: {
      normal: {
        stroke: '#FF441E',
      },
      hover: {
        stroke: '#FF441E',
      },
      select: {
        stroke: '#FF441E',
      },
    },
  },
  broken: {
    lineStyle: {
      normal: {
        stroke: '#C6C6CD',
        strokeWidth: 1,
      },
      hover: {
        stroke: '#C6C6CD',
        strokeWidth: 1,
      },
      select: {
        stroke: '#C6C6CD',
        strokeWidth: 1,
      },
    },
  },
  unknown: {},
};
