// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-require-imports */
import type { Config } from 'tailwindcss';

import { getTailwindContentByProject } from './utils';

export const tailwindcssConfig: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  important: '',
  safelist: [
    {
      pattern: /(gap-|grid-).+/,
      variants: ['sm', 'md', 'lg', 'xl', '2xl'],
    },
  ],
  corePlugins: {
    preflight: false,
  },
  presets: [
    require('@coze-arch/tailwind-config'),
    require('@cozeloop/tailwind-plugin/preset'),
  ],
  plugins: [
    require('@coze-arch/tailwind-config/coze'),
    require('@cozeloop/tailwind-plugin'),
  ],
};

export const createTailwindcssConfig = (config: {
  content?: string[];
  presets?: Config['presets'];
  plugins?: Config['plugins'];
}) => {
  if (config.content) {
    tailwindcssConfig.content = [
      ...(tailwindcssConfig.content as string[]),
      ...config.content.map(item => getTailwindContentByProject(item)),
    ];
  }
  if (Array.isArray(config.presets)) {
    tailwindcssConfig.presets = [
      ...(tailwindcssConfig.presets ?? []),
      ...config.presets,
    ];
  }
  if (Array.isArray(config.plugins)) {
    tailwindcssConfig.plugins = [
      ...(tailwindcssConfig.plugins ?? []),
      ...config.plugins,
    ];
  }
  return tailwindcssConfig;
};
