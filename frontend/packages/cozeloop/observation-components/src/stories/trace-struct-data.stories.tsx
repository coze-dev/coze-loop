// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import type { Meta, StoryObj } from '@storybook/react';

import { TraceStructData } from '@/features/trace-data';

import enUS from '../i18n/resource/en-US.json';
import { ConfigProvider } from '../config-provider';
import { mockTraceDetailData } from './mock-data';

const meta: Meta<typeof TraceStructData> = {
  title: 'Example/TraceStructData',
  component: TraceStructData,
  decorators: [
    Story => (
      <div className="min-h-[400px] h-auto w-full min-w-full p-4">
        <ConfigProvider
          sendEvent={(name, params) => {
            console.log(name, params);
          }}
          locale={{
            language: 'en',
            locale: enUS,
          }}
        >
          {Story()}
        </ConfigProvider>
      </div>
    ),
  ],
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {
    span: {
      description: 'Span data to render',
      control: { type: 'object' },
    },
    customSpanDefinition: {
      description: 'Custom span definition array',
      control: { type: 'object' },
    },
    spanRenderConfig: {
      description: 'Span render configuration',
      control: { type: 'object' },
    },
  },
} satisfies Meta<typeof TraceStructData>;

export default meta;
type Story = StoryObj<typeof meta>;

// Get first span from mock data
const firstSpan = mockTraceDetailData.spans[0];

export const Primary: Story = {
  args: {
    span: firstSpan as unknown as any,
  },
};
