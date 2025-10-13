/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
