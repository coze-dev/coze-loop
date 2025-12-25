// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type PropsWithChildren } from 'react';

import { SpanStatus } from '@cozeloop/api-schema/observation';
import {
  IconCozCheckMarkCircleFill,
  IconCozWarningCircleFillPalette,
} from '@coze-arch/coze-design/icons';

import { SpanType } from '@/features/trace-detail/types/params';
import { ReactComponent as IconSpanTool } from '@/assets/icons/icon-tool.svg';
import { ReactComponent as IconSpanRetriever } from '@/assets/icons/icon-retriever.svg';
import { ReactComponent as IconSpanPrompt } from '@/assets/icons/icon-prompt.svg';
import { ReactComponent as IconSpanPlugin } from '@/assets/icons/icon-plugin.svg';
import { ReactComponent as IconSpanParser } from '@/assets/icons/icon-parser.svg';
import { ReactComponent as IconSpanModel } from '@/assets/icons/icon-model.svg';
import { ReactComponent as IconSpanMemory } from '@/assets/icons/icon-memory.svg';
import { ReactComponent as IconSpanLoader } from '@/assets/icons/icon-loader.svg';
import { ReactComponent as IconSpanGraph } from '@/assets/icons/icon-graph.svg';
import { ReactComponent as IconSpanEmbedding } from '@/assets/icons/icon-embedding.svg';
import { ReactComponent as IconSpanDefault } from '@/assets/icons/icon-default.svg';
import { ReactComponent as IconSpanData } from '@/assets/icons/icon-data.svg';
import { ReactComponent as IconSpanBot } from '@/assets/icons/icon-bot.svg';
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
  icon?: (props: IconProps) => React.ReactNode;
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

export const CustomIcon = ({
  children,
}: PropsWithChildren<{ size?: 'small' | 'large' }>) => (
  <span className="w-[16px] h-[16px] text-[12px]">{children}</span>
);

export const NODE_CONFIG_MAP: Record<SpanType, NodeConfig> = {
  [SpanType.Unknown]: {
    color: '#9aa1f0',
    typeName: 'custom',
    character: 'C',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanDefault style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.Prompt]: {
    color: '#ffb016',
    typeName: 'prompt',
    character: 'Pr',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanPrompt style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.Model]: {
    color: '#b4baf6',
    typeName: 'model',
    character: 'Mo',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanModel style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.Parser]: {
    color: '#b9ecac',
    typeName: 'parser',
    character: 'Pa',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanParser style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.Embedding]: {
    color: '#d1aef4',
    typeName: 'embedding',
    character: 'Em',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanEmbedding style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.Memory]: {
    color: '#cfecac',
    typeName: 'memory',
    character: 'Me',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanMemory style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.Plugin]: {
    color: '#abcbf4',
    typeName: 'plugin',
    character: 'Pl',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanPlugin style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.Function]: {
    color: '#00BF40',
    typeName: 'function',
    character: 'Fn',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanTool
          style={{ width: '100%', height: '100%', color: 'white' }}
        />
      </CustomIcon>
    ),
  },

  [SpanType.Graph]: {
    color: '#00B2B2',
    typeName: 'graph',
    character: 'Gr',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanGraph
          style={{ width: '100%', height: '100%', color: 'white' }}
        />
      </CustomIcon>
    ),
  },

  [SpanType.Remote]: {
    color: '#cce7ff',
    typeName: 'remote',
    character: 'Rm',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanDefault style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.Loader]: {
    color: '#f0f0f5',
    typeName: 'loader',
    character: 'Ld',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanLoader style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.Transformer]: {
    color: '#ffdf99',
    typeName: 'transformer',
    character: 'Tf',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanDefault style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.VectorStore]: {
    color: '#ffd2d7',
    typeName: 'vector_store',
    character: 'VS',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanData style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.VectorRetriever]: {
    color: '#c1f2ef',
    typeName: 'vector_retriever',
    character: 'VR',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanRetriever style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },

  [SpanType.Agent]: {
    color: '#d1aef4',
    typeName: 'agent',
    character: 'Ag',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanBot style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
  [SpanType.CozeBot]: {
    color: '#5A4DED',
    typeName: 'bot',
    character: 'Bo',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanModel
          style={{ width: '100%', height: '100%', color: 'white' }}
        />
      </CustomIcon>
    ),
  },
  [SpanType.LLMCall]: {
    color: '#9aa1f0',
    typeName: 'llm_call',
    character: 'L',
    icon: ({ className, size }) => (
      <CustomIcon>
        <IconSpanModel style={{ width: '100%', height: '100%' }} />
      </CustomIcon>
    ),
  },
};

/** 虚拟根Broken节点id */
export const BROKEN_ROOT_SPAN_ID = '-10001';

/** 普通Broken节点id */
export const NORMAL_BROKEN_SPAN_ID = '-10002';

export const DEFAULT_KEY_SPAN_TYPE = ['model', 'tool', 'prompt'];
