// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { LanguageType } from '@cozeloop/api-schema/evaluation';

/** 前端使用语言类型 */
export enum CodeEvaluatorLanguageFE {
  Python = 'python',
  Javascript = 'javascript',
}

export enum SmallLanguageType {
  JS = 'js',
  Python = 'python',
}

/** LanguageType 服务端 -> 前端字段映射非标准, 手动转换一下 */
export const codeEvaluatorLanguageMap: Record<
  LanguageType & SmallLanguageType,
  string
> = {
  // Python -> python
  [LanguageType.Python]: 'python',
  // JS -> javascript
  [LanguageType.JS]: 'javascript',
  // 兼容一下服务端
  [SmallLanguageType.JS]: 'javascript',
  [SmallLanguageType.Python]: 'python',
};

/** LanguageType 前端 -> 服务端字段映射非标准, 手动转换一下 */
export const codeEvaluatorLanguageMapReverse: Record<string, LanguageType> = {
  // python -> Python
  python: LanguageType.Python,
  // javascript -> JS
  javascript: LanguageType.JS,
  [LanguageType.Python]: LanguageType.Python,
  [LanguageType.JS]: LanguageType.JS,
};

export const defaultJSCode = I18n.t('evaluate_code_evaluator_default_js_code');

export const defaultTestData = [
  {
    evaluate_dataset_fields: {
      input: {
        content_type: 'Text',
        text: I18n.t('evaluate_test_taiwan_area_question'),
      },
      reference_output: {
        content_type: 'Text',
        text: I18n.t('evaluate_taiwan_geography_overview'),
      },
    },
    evaluate_target_output_fields: {
      actual_output: {
        content_type: 'Text',
        text: I18n.t('evaluate_taiwan_geography_overview'),
      },
    },
    ext: {},
  },
];

export const MAX_SELECT_COUNT = 10;
