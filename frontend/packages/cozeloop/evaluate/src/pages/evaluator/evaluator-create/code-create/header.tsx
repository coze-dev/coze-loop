// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { I18n } from '@cozeloop/i18n-adapter';
import { RouteBackAction } from '@cozeloop/components';

export const CodeCreateHeader = () => (
  <div className="px-6 flex-shrink-0 py-3 h-[56px] flex flex-row items-center">
    <RouteBackAction defaultModuleRoute="evaluation/evaluators" />
    <span className="ml-2 text-[18px] font-medium coz-fg-plus">
      {I18n.t('create_evaluator')}
    </span>
  </div>
);
