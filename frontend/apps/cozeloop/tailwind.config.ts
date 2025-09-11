// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createTailwindcssConfig } from '@cozeloop/tailwind-config';

type TailwindConfig = ReturnType<typeof createTailwindcssConfig>;

export default createTailwindcssConfig({
  content: [
    '@coze-arch/coze-design',
    '@cozeloop/components',
    '@cozeloop/auth-pages',
    '@cozeloop/prompt-pages',
    '@cozeloop/evaluate-components',
    '@cozeloop/evaluate',
    '@cozeloop/evaluate-pages',
    '@cozeloop/observation-pages',
    '@cozeloop/trace-struct-data',
    '@cozeloop/trace-list',
    '@cozeloop/observation-component-adapter',
    '@cozeloop/tag-pages',
    '@cozeloop/observation-component',
    '@cozeloop/tag-components',
    '@cozeloop/prompt-components',
  ],
}) as TailwindConfig;
