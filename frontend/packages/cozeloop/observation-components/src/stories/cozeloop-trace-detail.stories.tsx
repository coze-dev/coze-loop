// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import type { Meta, StoryObj } from '@storybook/react';

import { CozeloopTraceDetail } from '@/features/trace-detail';

import enUS from '../i18n/resource/en-US.json';
import { ConfigProvider } from '../config-provider';
import { mockTraceDetailData } from './mock-data';

const mockGetTraceDetailData = () => Promise.resolve(mockTraceDetailData);

const meta: Meta<typeof CozeloopTraceDetail> = {
  title: 'Example/CozeloopTraceDetail',
  component: CozeloopTraceDetail,
  decorators: [
    Story => (
      <div className="min-h-[600px] h-[600px] w-full min-w-full">
        <ConfigProvider
          sendEvent={(name, params) => {
            console.log(name, params);
          }}
          locale={{
            language: 'en',
            locale: enUS,
          }}
          bizId="cozeloop"
          envConfig={{
            isDev: true,
            isOverSea: false,
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
  argTypes: {},
} satisfies Meta<typeof CozeloopTraceDetail>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    getTraceDetailData: mockGetTraceDetailData as unknown as any,
    layout: 'horizontal',
  },
};

export const WithHeader: Story = {
  args: {
    ...Primary.args,
    headerConfig: {
      visible: true,
      showClose: true,
      onClose: () => {
        console.log('Close clicked');
        alert('Close button clicked');
      },
      minColWidth: 200,
      customRender: span => (
        <div className="flex items-center gap-2">
          <span className="font-semibold">Custom Header:</span>
          <span>{span?.span_name || 'No span selected'}</span>
        </div>
      ),
    },
  },
};

export const WithSpanDetailConfig: Story = {
  args: {
    ...Primary.args,
    spanDetailConfig: {
      showTags: true,
      baseInfoPosition: 'right',
      minColWidth: 300,
      maxColNum: 3,
      extraTagList: [
        {
          title: 'Custom Tag',
          item: span => `Custom: ${span.custom_tags?.model_name || 'N/A'}`,
          enableCopy: true,
        },
        {
          title: 'Execution Time',
          item: span => `${span.duration}ms`,
          enableCopy: true,
        },
      ],
    },
  },
};

export const WithSwitchConfig: Story = {
  args: {
    ...Primary.args,
    switchConfig: {
      canSwitchPre: true,
      canSwitchNext: true,
      onSwitch: action => {
        console.log('Switch:', action);
        alert(`Switch to ${action} span`);
      },
    },
  },
};
