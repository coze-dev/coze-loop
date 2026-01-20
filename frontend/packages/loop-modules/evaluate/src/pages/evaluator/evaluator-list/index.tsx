// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useSearchParams } from 'react-router-dom';
import { useState, useEffect } from 'react';

import { sendEvent, EVENT_NAMES } from '@cozeloop/tea-adapter';
import { I18n } from '@cozeloop/i18n-adapter';
import { LoopTabs, PrimaryPage } from '@cozeloop/components';

import { EvaluatorSource } from '../evaluator-template/types';
import EvaluatorListPage from './evaluator-list-page';
import { BuiltinEvaluatorList } from './builtin-evaluator-list';

const ACTIVE_QUERY_KEY = 'active_tab';

function EvaluatorIndexPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeKey, setActiveKey] = useState(
    searchParams.get(ACTIVE_QUERY_KEY) || 'custom',
  );

  useEffect(() => {
    if (!searchParams.get(ACTIVE_QUERY_KEY)) {
      setSearchParams({ [ACTIVE_QUERY_KEY]: 'custom' }, { replace: true });
    }
  }, [searchParams, setSearchParams]);

  const handleTabChange = (key: string) => {
    if (key === EvaluatorSource.BUILTIN) {
      sendEvent(EVENT_NAMES.cozeloop_pre_evaluator_entry_click);
    }
    setActiveKey(key);
    setSearchParams({ [ACTIVE_QUERY_KEY]: key }, { replace: true });
  };
  return (
    <PrimaryPage pageTitle={I18n.t('evaluator')} contentClassName="!px-0">
      <div className="flex flex-col h-full overflow-hidden">
        <LoopTabs
          activeKey={activeKey}
          type="card"
          onChange={handleTabChange}
          tabList={[
            { tab: I18n.t('self_built_evaluator'), itemKey: 'custom' },
            { tab: I18n.t('preset_evaluator'), itemKey: 'builtin' },
          ]}
        />
        <div className="flex-1 overflow-hidden">
          {activeKey === EvaluatorSource.CUSTOM && (
            <EvaluatorListPage className="px-6" />
          )}
          {activeKey === EvaluatorSource.BUILTIN && <BuiltinEvaluatorList />}
        </div>
      </div>
    </PrimaryPage>
  );
}

export default EvaluatorIndexPage;
