// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback } from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { CodeEditor, type Monaco } from '@cozeloop/components';
import {
  type CommonFieldProps,
  Select,
  withField,
} from '@coze-arch/coze-design';

import { CodeEvaluatorLanguageFE } from '@/constants';

import type { BaseFuncExecutorProps } from '../types';

const languageOptions = [
  { label: 'JavaScript', value: CodeEvaluatorLanguageFE.Javascript },
  { label: 'Python', value: CodeEvaluatorLanguageFE.Python },
];

const handleEditorDidMount = (
  monaco: Monaco,
  language?: CodeEvaluatorLanguageFE,
) => {
  // 配置 Python 语言服务
  if (language === CodeEvaluatorLanguageFE.Python) {
    // 设置 Python 特定的配置
    monaco.languages.setLanguageConfiguration('python', {
      comments: {
        lineComment: '#',
        blockComment: ['"""', '"""'],
      },
      brackets: [
        ['{', '}'],
        ['[', ']'],
        ['(', ')'],
      ],
      autoClosingPairs: [
        { open: '{', close: '}' },
        { open: '[', close: ']' },
        { open: '(', close: ')' },
        { open: '"', close: '"', notIn: ['string'] },
        { open: "'", close: "'", notIn: ['string', 'comment'] },
      ],
      surroundingPairs: [
        { open: '{', close: '}' },
        { open: '[', close: ']' },
        { open: '(', close: ')' },
        { open: '"', close: '"' },
        { open: "'", close: "'" },
      ],
    });
  }
};

const getDefaultCode = (language: CodeEvaluatorLanguageFE): string => {
  if (language === CodeEvaluatorLanguageFE.Javascript) {
    return `function exec_evaluation(eval_input) {
  // 在这里编写你的评估逻辑
  // input: 输入数据
  // output: 模型输出
  // expected: 期望输出

  // 返回评估结果对象
  return {
    score: 1.0, // 分数 (0-1)
    description: "评估通过"
  };
}`;
  }

  return `def exec_evaluation(eval_input):
    """
    在这里编写你的评估逻辑
    input: 输入数据
    output: 模型输出
    expected: 期望输出

    返回评估结果字典
    """
    return {
        "score": 1.0,  # 分数 (0-1)
        "description": "评估通过"
    }`;
};

// 基础组件实现
export const BaseFuncExecutor: React.FC<BaseFuncExecutorProps> = ({
  value,
  onChange,
  disabled,
  editorHeight,
}) => {
  const { language, code } = value || {};
  const handleLanguageChange = useCallback(
    (newLanguage: CodeEvaluatorLanguageFE) => {
      // 切换语言, 重置默认代码
      const defaultCode = getDefaultCode(newLanguage);
      onChange?.({ language: newLanguage, code: defaultCode });
    },
    [onChange],
  );

  const handleCodeChange = useCallback(
    (newValue: string | undefined) => {
      onChange?.({ ...value, code: newValue || '' });
    },
    [onChange, value],
  );

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      {/* start_aigc */}
      <div
        className="flex items-center h-[44px] py-2 px-3"
        style={{
          borderBottom: '1px solid rgba(82, 100, 154, 0.13)',
        }}
      >
        <h3 className="text-sm font-medium text-gray-900 mr-4">
          {I18n.t('evaluate_func_body')}
        </h3>
        <Select
          value={language}
          onChange={v => handleLanguageChange(v as CodeEvaluatorLanguageFE)}
          className="w-[120px] h-[24px] min-h-[24px]"
          size="small"
          disabled={true}
        >
          {languageOptions.map(option => (
            <Select.Option key={option.value} value={option.value}>
              {option.label}
            </Select.Option>
          ))}
        </Select>
      </div>
      {/* end_aigc */}

      {/* Code Editor */}
      <div className="flex-1 rounded-b-lg">
        <CodeEditor
          language={language}
          value={code}
          onChange={handleCodeChange}
          onMount={(_, monaco) => handleEditorDidMount(monaco, language)}
          options={{
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
            wordWrap: 'on',
            fontSize: 12,
            lineNumbers: 'on',
            folding: true,
            automaticLayout: true,
            readOnly: disabled,
          }}
          theme="vs-light"
          height={editorHeight || '500px'}
        />
      </div>
    </div>
  );
};

// 使用withField包装组件
const FuncExecutor: React.ComponentType<
  BaseFuncExecutorProps & CommonFieldProps
> = withField(BaseFuncExecutor);

export default FuncExecutor;
