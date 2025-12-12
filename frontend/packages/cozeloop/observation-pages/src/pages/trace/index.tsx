// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import zhCH from '@cozeloop/observation-components/zh-CN';
import enUS from '@cozeloop/observation-components/en-US';
import { ConfigProvider } from '@cozeloop/observation-components';
import { I18n } from '@cozeloop/i18n-adapter';
import { PrimaryPage } from '@cozeloop/components';

import { TracesRender } from './trace-render';

const TracesPage = () => {
  const lang = I18n.language === 'zh-CN' ? zhCH : enUS;

  return (
    <div className="h-full max-h-full w-full flex-1 max-w-full overflow-hidden !min-w-[980px] flex flex-col">
      <PrimaryPage pageTitle="Trace" className="!pb-0">
        <ConfigProvider
          bizId="coze_loop_open"
          locale={{
            language: I18n.lang,
            locale: lang,
          }}
        >
          <TracesRender />
        </ConfigProvider>
      </PrimaryPage>
    </div>
  );
};

export { TracesPage };
