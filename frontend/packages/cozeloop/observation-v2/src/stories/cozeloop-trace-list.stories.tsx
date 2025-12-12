/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/naming-convention */
import { BrowserRouter } from 'react-router-dom';

import type { Meta, StoryObj } from '@storybook/react';
import { FieldType, QueryType } from '@cozeloop/api-schema/observation';

import { BUILD_IN_COLUMN } from '@/features/trace-list/types';
import { PresetRange } from '@/features/trace-list/constants/time';
import { CozeloopTraceList } from '@/features/trace-list';

import enUS from '../i18n/resource/en-US.json';
import { ConfigProvider } from '../config-provider';
import { mockTraceListData } from './mock-data';

// 模拟字段元信息函数
const mockGetFieldMetas = () =>
  Promise.resolve({
    input: {
      filter_types: [QueryType.Match, QueryType.Exist, QueryType.NotExist],
      support_customizable_option: true,
    },
    output: {
      filter_types: [QueryType.Match, QueryType.Exist, QueryType.NotExist],
      support_customizable_option: true,
      value_type: FieldType.String,
    },
    span_name: {
      filter_types: [QueryType.Match, QueryType.Exist, QueryType.NotExist],
      support_customizable_option: true,
      value_type: FieldType.String,
    },
    status: {
      filter_types: [QueryType.Match, QueryType.Exist, QueryType.NotExist],
      support_customizable_option: false,
      value_type: FieldType.String,
    },
    tokens: {
      filter_types: [QueryType.Gte, QueryType.Exist, QueryType.NotExist],
      support_customizable_option: false,
      value_type: FieldType.Long,
    },
  });

// 模拟获取 trace 列表数据函数
const mockGetTraceList = () => Promise.resolve(mockTraceListData);

const meta: Meta<typeof CozeloopTraceList> = {
  title: 'Example/CozeloopTraceList',
  component: CozeloopTraceList,
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
} satisfies Meta<typeof CozeloopTraceList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    columnsConfig: {
      columns: [
        BUILD_IN_COLUMN.TraceId,
        BUILD_IN_COLUMN.SpanName,
        BUILD_IN_COLUMN.Status,
        BUILD_IN_COLUMN.StartTime,
        BUILD_IN_COLUMN.Latency,
      ],
    },
    getFieldMetas: mockGetFieldMetas as unknown as any,
    getTraceList: mockGetTraceList as unknown as any,
  },
};

export const WithCustomColumns: Story = {
  args: {
    ...Primary.args,
    columnsConfig: {
      columns: [
        BUILD_IN_COLUMN.TraceId,
        BUILD_IN_COLUMN.SpanName,
        BUILD_IN_COLUMN.Input,
        BUILD_IN_COLUMN.Output,
        BUILD_IN_COLUMN.Tokens,
        BUILD_IN_COLUMN.Status,
      ],
    },
  },
};

export const WithFilters: Story = {
  args: {
    ...Primary.args,
    defaultFilters: {
      selectedSpanType: 'model',
      timestamps: [1753932434000, 1753932436000],
      presetTimeRange: PresetRange.Day3,
    },
    onFiltersChange: filters => {
      console.log('Filters changed:', filters);
    },
  },
};

export const WithRowClick: Story = {
  args: {
    ...Primary.args,
    onRowClick: (span, index) => {
      console.log('Row clicked:', span, index);
      alert(`Clicked on span: ${span.span_name} (${span.span_id})`);
    },
  },
};
