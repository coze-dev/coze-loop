// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createRoot } from 'react-dom/client';
import { initIntl } from '@cozeloop/i18n-adapter';
import { dynamicImportMdBoxStyle } from '@coze-arch/bot-md-box-adapter/style';
import { pullFeatureFlags, type FEATURE_FLAGS } from '@coze-arch/bot-flags';

export async function render() {
  await Promise.all([
    initIntl({
      fallbackLng: ['zh-CN', 'en-US'],
    }),
    pullFeatureFlags({
      timeout: 1000 * 4,
      fetchFeatureGating: () => Promise.resolve({} as unknown as FEATURE_FLAGS),
    }),
    dynamicImportMdBoxStyle(),
  ]);

  const dom = document.getElementById('cozeloop-root');

  if (dom) {
    const { App } = await import('./app');
    const root = createRoot(dom);
    root.render(<App />);
  }
}

render();
