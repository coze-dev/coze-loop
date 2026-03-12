// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createRsbuildConfig } from '@cozeloop/rsbuild-config';

export type RsbuildConfig = ReturnType<typeof createRsbuildConfig>;

const port = 8685;
const mockServerPort = 9406;

export default createRsbuildConfig({
  server: { port, strictPort: true },
  dev: {
    assetPrefix: `http://localhost:${port}`,
    client: {
      port: `${port}`,
      host: 'localhost',
      protocol: 'ws',
    },
  },
  html: {
    title: 'Coze Loop',
    template: './src/assets/template.html',
    favicon: './src/assets/images/coze.svg',
    crossorigin: 'anonymous',
  },
}) as RsbuildConfig;
