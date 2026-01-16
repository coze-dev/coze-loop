// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useEffect, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { CodeEditor } from '@cozeloop/components';
import { type EvaluatorVersion } from '@cozeloop/api-schema/evaluation';
import { IconCozArrowRight } from '@coze-arch/coze-design/icons';
import { Loading } from '@coze-arch/coze-design';

import { CodeEvaluatorLanguageFE, codeEvaluatorLanguageMap } from '@/constants';
import { type BaseFuncExecutorValue } from '@/components/evaluator-code/types';

interface CodeEvaluatorContentProps {
  versionDetail?: EvaluatorVersion;
  loading?: boolean;
}

/**
 * 将 versionDetail 中的 code_evaluator 数据转换为组件需要的数据格式
 */
function transformVersionDetailToCodeEvaluator(
  versionDetail: EvaluatorVersion,
): BaseFuncExecutorValue {
  const codeEvaluator = versionDetail?.evaluator_content?.code_evaluator;

  if (!codeEvaluator) {
    return {
      language: CodeEvaluatorLanguageFE.Javascript,
      code: '',
    };
  }

  const { language_type, code_content } = codeEvaluator;

  return {
    language: language_type
      ? (codeEvaluatorLanguageMap[language_type] as CodeEvaluatorLanguageFE)
      : CodeEvaluatorLanguageFE.Javascript,
    code: code_content || '',
  };
}

export function CodeEvaluatorContent({
  versionDetail,
  loading,
}: CodeEvaluatorContentProps) {
  const [open, setOpen] = useState(false);

  // 内部转换数据
  const codeEvaluatorValue = transformVersionDetailToCodeEvaluator(
    versionDetail as EvaluatorVersion,
  );
  const { language, code } = codeEvaluatorValue;

  // 加载完成后打开
  useEffect(() => {
    if (!loading) {
      setOpen(true);
    }
  }, [loading]);

  return (
    <div className="mb-4">
      {/* 可折叠的标题栏 */}
      <div
        className="h-5 my-1 flex flex-row items-center cursor-pointer text-sm coz-fg-primary font-semibold"
        onClick={() => setOpen(pre => !pre)}
      >
        {I18n.t('evaluate_code_evaluator_detail')}
        <IconCozArrowRight
          className={classNames(
            'h-4 w-4 ml-2 coz-fg-plus transition-transform',
            open ? 'rotate-90' : '',
          )}
        />
      </div>
      {open && loading ? (
        <div className="h-[84px] w-full flex items-center justify-center">
          <Loading
            className="!w-full"
            size="large"
            label={I18n.t('evaluate_loading_code_evaluator_detail')}
            loading={true}
          />
        </div>
      ) : null}

      {/* 内容区域 */}
      {open && !loading ? (
        <div className="mt-4">
          {/* 代码编辑器 */}
          <div className="h-[300px] border border-gray-200 rounded-lg overflow-hidden">
            <CodeEditor
              language={language}
              value={code}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                fontSize: 12,
                lineNumbers: 'on',
                folding: true,
                automaticLayout: true,
                readOnly: true,
              }}
              theme="vs-light"
              height="300px"
            />
          </div>
        </div>
      ) : null}
    </div>
  );
}
