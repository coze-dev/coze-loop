// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import { CodeEditor } from '@cozeloop/components';
import { type EvaluatorTemplate } from '@cozeloop/api-schema/evaluation';
import { Select } from '@coze-arch/coze-design';

import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMapReverse,
} from '@/constants';

const editorOptions = {
  minimap: { enabled: false },
  scrollBeyondLastLine: false,
  fontSize: 12,
  folding: true,
  automaticLayout: true,
  readOnly: true,
};

const languageOptions = [
  { label: 'JavaScript', value: CodeEvaluatorLanguageFE.Javascript },
  { label: 'Python', value: CodeEvaluatorLanguageFE.Python },
];

interface EvaluatorTemplateCodeDetailProps {
  template: EvaluatorTemplate;
  onCodeLangChange?: (lang: CodeEvaluatorLanguageFE) => void;
}

export function EvaluatorTemplateCodeDetail(
  props: EvaluatorTemplateCodeDetailProps,
) {
  const { template, onCodeLangChange } = props;
  const [language, setLanguage] = useState<CodeEvaluatorLanguageFE>(
    CodeEvaluatorLanguageFE.Python,
  );
  const [code, setCode] = useState('');

  useEffect(() => {
    const codeContent =
      template.evaluator_content?.code_evaluator?.lang_2_code_content?.[
        codeEvaluatorLanguageMapReverse[language]
      ] || '';
    setCode(codeContent);
  }, [template, language]);

  useEffect(() => {
    onCodeLangChange?.(language);
  }, [language]);

  return (
    <div className="flex flex-col h-full border border-solid coz-stroke-primary rounded-lg">
      <div
        className="flex items-center justify-between h-[44px] py-2 px-3"
        style={{
          borderBottom: '1px solid rgba(82, 100, 154, 0.13)',
        }}
      >
        <h3 className="text-sm font-medium text-gray-900 mr-4">Code</h3>
        <Select
          value={language}
          onChange={v => setLanguage(v as CodeEvaluatorLanguageFE)}
          className="w-[120px] h-[24px] min-h-[24px]"
          size="small"
        >
          {languageOptions.map(option => (
            <Select.Option key={option.value} value={option.value}>
              {option.label}
            </Select.Option>
          ))}
        </Select>
      </div>

      <div className="flex-1 rounded-b-lg">
        <CodeEditor
          language={language}
          value={code}
          options={editorOptions}
          theme="vs-light"
          height="500px"
        />
      </div>
    </div>
  );
}
