import path from 'path';

import { defineConfig } from '@rslib/core';

export default defineConfig({
  lib: [
    {
      format: 'esm',
      syntax: 'es6',
      output: {
        distPath: {
          root: './dist/es',
        },
        target: 'web',
      },
      bundle: true,
      dts: {
        build: true,
      },
      source: {
        entry: {
          index: './src/index.ts',
          observation: './src/api/observability/index.ts',
          evaluation: './src/api/evaluation/index.ts',
          data: './src/api/data/index.ts',
          llmManage: './src/api/llm-manage/index.ts',
          foundation: './src/api/foundation/index.ts',
          prompt: './src/api/prompt/index.ts',
        },
        tsconfigPath: path.resolve(__dirname, 'tsconfig.build.json'),
      },
    },
  ],
});
