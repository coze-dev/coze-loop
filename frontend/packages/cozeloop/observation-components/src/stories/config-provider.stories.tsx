// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import type { Meta, StoryObj } from '@storybook/react';

import { ConfigProvider } from '../config-provider';

const meta = {
  title: 'Example/ConfigProvider',
  component: ConfigProvider,
  parameters: {
    layout: 'centered',
  },
  tags: ['autodocs'],
  argTypes: {},
} satisfies Meta<typeof ConfigProvider>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: {
    children: <div>Hello World</div>,
    sendEvent: (name, params) => {
      console.log('sendEvent', name, params);
    },
  },
};
