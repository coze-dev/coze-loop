// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from '@coze-arch/vitest-config';

export default defineConfig({
  dirname: __dirname,
  preset: 'web',
  test: {
    coverage: {
      all: true,
      exclude: ['src/index.ts', 'src/logger/index.ts'],
    },
  },
});
