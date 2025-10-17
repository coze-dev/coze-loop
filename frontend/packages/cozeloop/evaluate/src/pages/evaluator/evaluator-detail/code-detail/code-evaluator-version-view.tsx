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

import { CodeEditor } from '@cozeloop/components';
import {
  type EvaluatorVersion,
  type LanguageType,
} from '@cozeloop/api-schema/evaluation';

import { codeEvaluatorLanguageMap } from '@/constants';
import { I18n } from '@cozeloop/i18n-adapter';

interface Props {
  version: EvaluatorVersion;
}

{
  /* start_aigc */
}
export function CodeEvaluatorVersionView({ version }: Props) {
  const codeEvaluator = version.evaluator_content?.code_evaluator;
  // 从API结构中获取数据
  const language = codeEvaluator?.language_type as LanguageType;
  const code = codeEvaluator?.code_content;
  const templateName = codeEvaluator?.code_template_name;

  const langText = codeEvaluatorLanguageMap[language];

  return (
    <div className="space-y-6">
      <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus">
        {I18n.t('config_info')}
      </div>

      {/* Code 评估器配置 */}
      <div className="space-y-4">
        {/* 编程语言 */}
        {language ? (
          <div className="space-y-2">
            <div className="text-[12px] font-medium coz-fg-secondary">
              {I18n.t('evaluate_programming_language')}
            </div>
            <div className="text-[14px] coz-fg-plus">
              {langText
                ? langText.charAt(0).toUpperCase() + langText.slice(1)
                : ''}
            </div>
          </div>
        ) : null}

        {/* 模板名称 */}
        {templateName ? (
          <div className="space-y-2">
            <div className="text-[12px] font-medium coz-fg-secondary">
              {I18n.t('evaluate_used_template')}
            </div>
            <div className="text-[14px] coz-fg-plus">{templateName}</div>
          </div>
        ) : null}

        {/* 代码内容 */}
        {code ? (
          <div className="space-y-2">
            <div className="text-[12px] font-medium coz-fg-secondary">
              {I18n.t('evaluate_code_content')}
            </div>
            <div
              className="border border-gray-200 rounded-lg overflow-hidden"
              style={{ height: 800 }}
            >
              <CodeEditor
                value={code}
                language={langText}
                options={{
                  minimap: { enabled: false },
                  scrollBeyondLastLine: false,
                  readOnly: true,
                  wordWrap: 'on',
                }}
              />
            </div>
          </div>
        ) : null}
      </div>

      {/* 如果没有配置信息，显示提示 */}
      {!language && !code && (
        <div className="text-center py-8 text-gray-500">
          {I18n.t('evaluate_no_config_info')}
        </div>
      )}
    </div>
  );
}
{
  /* end_aigc */
}
