// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type PropsWithChildren, type ReactNode } from 'react';

import {
  IconCozBrace,
  IconCozCheckMarkCircleFill,
  IconCozWarningCircleFillPalette,
  IconCozDocumentFill,
  IconCozBotFill,
  IconCozCodeFill,
  IconCozWorkflowFill,
} from '@coze-arch/coze-design/icons';

import { ReactComponent as IconSpanPluginTool } from '@/icons/icon-plugin-tool.svg';
import { ReactComponent as IconSpanKnowledge } from '@/icons/icon-knowledge.svg';
import { ReactComponent as IconSpanDatabase } from '@/icons/icon-database.svg';
import { ReactComponent as IconSpanAgent } from '@/icons/icon-agent.svg';

import { SpanStatus, SpanType } from '../typings/params';

export const SPAN_STATUS_MAP = {
  [SpanStatus.Success]: {
    icon: IconCozCheckMarkCircleFill,
    text: 'Success',
    className: 'success',
  },
  [SpanStatus.Error]: {
    icon: IconCozWarningCircleFillPalette,
    text: 'Error',
    className: 'error',
  },
};

interface IconProps {
  className?: string;
  size?: 'small' | 'large';
}
export interface NodeConfig {
  color: string;
  title?: string;
  typeName: string;
  /** 节点标识，用于渲染图标 */
  character: string;
  icon?: (props: IconProps) => ReactNode;
}

export const CustomIconWrapper = ({
  color,
  children,
  size = 'small',
}: PropsWithChildren<{ color: string; size?: 'small' | 'large' }>) => (
  <span
    className="w-full h-full inline-flex items-center justify-center text-white font-semibold text-[10px]"
    style={{
      background: color,
      borderRadius: size === 'small' ? '4px' : '8px',
    }}
  >
    {children}
  </span>
);

export const NODE_CONFIG_MAP: Record<SpanType, NodeConfig> = {
  [SpanType.Unknown]: {
    color: '#9aa1f0',
    typeName: 'custom',
    character: 'C',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#9aa1f0" size={size}>
        <IconCozBrace
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.Prompt]: {
    color: '#ffb016',
    typeName: 'prompt',
    character: 'Pr',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#ffb016" size={size}>
        <IconCozDocumentFill
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },

  [SpanType.Model]: {
    color: '#b4baf6',
    typeName: 'model',
    character: 'Mo',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#5A4DED" size={size}>
        <IconCozBotFill
          style={{ width: '100%', height: '100%', color: 'white' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.Parser]: {
    color: '#b9ecac',
    typeName: 'parser',
    character: 'Pa',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#b9ecac" size={size}>
        <IconSpanPluginTool
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.Embedding]: {
    color: '#d1aef4',
    typeName: 'embedding',
    character: 'Em',
  },
  [SpanType.Memory]: {
    color: '#cfecac',
    typeName: 'memory',
    character: 'Me',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#cfecac" size={size}>
        <IconSpanKnowledge
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.Plugin]: {
    color: '#abcbf4',
    typeName: 'plugin',
    character: 'Pl',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#abcbf4" size={size}>
        <IconSpanPluginTool
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },

  [SpanType.Function]: {
    color: '#00BF40',
    typeName: 'function',
    character: 'Fn',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#00BF40" size={size}>
        <IconCozWorkflowFill
          style={{ width: '100%', height: '100%', color: 'white' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },

  [SpanType.Graph]: {
    color: '#00B2B2',
    typeName: 'graph',
    character: 'Gr',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#00B2B2" size={size}>
        <IconCozCodeFill
          style={{ width: '100%', height: '100%', color: 'white' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },

  [SpanType.Remote]: {
    color: '#cce7ff',
    typeName: 'remote',
    character: 'Rm',
  },

  [SpanType.Loader]: {
    color: '#f0f0f5',
    typeName: 'loader',
    character: 'Ld',
  },

  [SpanType.Transformer]: {
    color: '#ffdf99',
    typeName: 'transformer',
    character: 'Tf',
  },

  [SpanType.VectorStore]: {
    color: '#ffd2d7',
    typeName: 'vector_store',
    character: 'VS',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#ffd2d7" size={size}>
        <IconSpanDatabase
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },

  [SpanType.VectorRetriever]: {
    color: '#c1f2ef',
    typeName: 'vector_retriever',
    character: 'VR',
  },

  [SpanType.Agent]: {
    color: '#d1aef4',
    typeName: 'agent',
    character: 'Ag',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#d1aef4" size={size}>
        <IconSpanAgent
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.CozeBot]: {
    color: '#5A4DED',
    typeName: 'bot',
    character: 'Bo',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#5A4DED" size={size}>
        <IconCozBotFill
          style={{ width: '100%', height: '100%', color: 'white' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
  [SpanType.LLMCall]: {
    color: '#9aa1f0',
    typeName: 'llm_call',
    character: 'L',
    icon: ({ className, size }) => (
      <CustomIconWrapper color="#9aa1f0" size={size}>
        <IconCozBrace
          style={{ width: '100%', height: '100%' }}
          className={className}
        />
      </CustomIconWrapper>
    ),
  },
};

/** 虚拟根Broken节点id */
export const BROKEN_ROOT_SPAN_ID = '-10001';

/** 普通Broken节点id */
export const NORMAL_BROKEN_SPAN_ID = '-10002';
