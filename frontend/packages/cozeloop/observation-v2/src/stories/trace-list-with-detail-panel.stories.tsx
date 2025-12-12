/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import { BrowserRouter } from 'react-router-dom';

import type { Meta, StoryObj } from '@storybook/react';

import { CozeloopTraceListWithDetailPanel } from '@/trace-list-with-detail-panel';
import { BUILD_IN_COLUMN } from '@/features/trace-list/types';

import enUS from '../i18n/resource/en-US.json';
import { ConfigProvider } from '../config-provider';
import { mockTraceListData, mockTraceDetailData } from './mock-data';

const mockGetTraceDetailData = () => Promise.resolve(mockTraceDetailData);

const mockGetTraceList = () => Promise.resolve(mockTraceListData);

const meta: Meta<typeof CozeloopTraceListWithDetailPanel> = {
  title: 'Example/CozeloopTraceListWithDetailPanel',
  component: CozeloopTraceListWithDetailPanel,
  decorators: [
    Story => (
      <div className="min-h-[600px] h-[600px]">
        <BrowserRouter>
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
        </BrowserRouter>
      </div>
    ),
  ],
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {},
} satisfies Meta<typeof CozeloopTraceListWithDetailPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    bizId: 'cozeloop',
    columnsConfig: {
      columns: [
        BUILD_IN_COLUMN.TraceId,
        BUILD_IN_COLUMN.SpanName,
        BUILD_IN_COLUMN.Status,
        BUILD_IN_COLUMN.StartTime,
        BUILD_IN_COLUMN.Latency,
      ],
    },
    // @ts-expect-error not fix
    getFieldMetas: () =>
      Promise.resolve({
        input: {
          filter_types: ['match', 'exist', 'not_exist'],
          support_customizable_option: true,
          value_type: 'string',
        },
      }),
    getTraceDetailData: mockGetTraceDetailData as unknown as any,
    getTraceList: mockGetTraceList as unknown as any,
    customParams: {
      spanRenderConfig: {
        // enableCopy: () => false,
        enableCopy: (content: any, type: 'input' | 'output') => {
          if (typeof content?.input === 'string' && type === 'input') {
            const input = JSON.parse(content.input);
            if (input?.action_name === 'content_plugin') {
              return false;
            }
          }
          return true;
        },
      },
    },
  },
};
