import { type Evaluator, LanguageType } from '@cozeloop/api-schema/evaluation';
import { IconCozTemplate, IconCozExpand } from '@coze-arch/coze-design/icons';
import { useFormState, Button } from '@coze-arch/coze-design';

import {
  codeEvaluatorLanguageMap,
  codeEvaluatorLanguageMapReverse,
  CodeEvaluatorLanguageFE,
  defaultTestData,
  defaultJSCode,
} from '@/constants';
import {
  TestDataSource,
  type CodeEvaluatorValue,
} from '@/components/evaluator-code/types';
import { BaseCodeEvaluatorConfig } from '@/components/evaluator-code';
import { I18n } from '@cozeloop/i18n-adapter';

interface CodeEvaluatorConfigFieldProps {
  disabled?: boolean;
  refreshEditorModelKey?: number;
  debugLoading?: boolean;
  onOpenTemplateModal?: () => void;
  templateInfo?: {
    key: string;
    name: string;
    lang: string;
  } | null;
  onFullscreenToggle?: () => void;
  editorHeight?: string;
}

{
  /* start_aigc */
}
/**
 * 将 API 数据转换为组件期望的数据结构
 */
export function transformApiToComponent(evaluator?: Evaluator): {
  config: CodeEvaluatorValue;
} {
  if (!evaluator) {
    return {
      config: {
        funcExecutor: {
          language: CodeEvaluatorLanguageFE.Javascript,
          code: defaultJSCode,
        },
        testData: {
          source: TestDataSource.Custom,
          customData: defaultTestData[0],
        },
      },
    };
  }
  const codeEvaluator =
    evaluator.current_version?.evaluator_content?.code_evaluator;

  const { language_type, code_content } = codeEvaluator as {
    language_type?: string | LanguageType;
    code_content?: string;
  };

  return {
    config: {
      funcExecutor: {
        language: language_type
          ? (codeEvaluatorLanguageMap[language_type] as CodeEvaluatorLanguageFE)
          : CodeEvaluatorLanguageFE.Javascript,
        code: code_content || '',
      },
      testData: {
        source: TestDataSource.Custom,
        customData: defaultTestData[0],
      },
    },
  };
}

/**
 * 将组件数据转换为 API 期望的数据结构
 */
export function transformComponentToApi(
  componentData: CodeEvaluatorValue,
): Record<string, unknown> {
  const { funcExecutor } = componentData;

  return {
    language_type: funcExecutor?.language
      ? codeEvaluatorLanguageMapReverse[funcExecutor.language]
      : LanguageType.JS,
    code_content: funcExecutor?.code || '',
  };
}

export function CodeEvaluatorConfigField({
  disabled,
  refreshEditorModelKey,
  debugLoading,
  onOpenTemplateModal,
  templateInfo,
  onFullscreenToggle,
  editorHeight,
}: CodeEvaluatorConfigFieldProps) {
  const { values: formValue } = useFormState();
  const { config = {} } = formValue;

  return (
    <div className="flex flex-col h-full">
      <div className="h-[28px] mb-3 text-[16px] leading-7 font-medium coz-fg-plus flex flex-row items-center justify-between">
        <span>{I18n.t('evaluate_config')}</span>
        <div className="flex items-center gap-2">
          {onFullscreenToggle ? (
            <Button
              size="mini"
              color="secondary"
              className="!coz-fg-hglt !px-[3px] !h-5"
              icon={<IconCozExpand />}
              onClick={onFullscreenToggle}
            >
              全屏
            </Button>
          ) : null}
          {onOpenTemplateModal ? (
            <Button
              size="mini"
              color="secondary"
              className="!coz-fg-hglt !px-[3px] !h-5"
              icon={<IconCozTemplate />}
              onClick={onOpenTemplateModal}
            >
              {`选择模板${templateInfo?.name ? `(${templateInfo.name})` : ''}`}
            </Button>
          ) : null}
        </div>
      </div>
      <BaseCodeEvaluatorConfig
        key={refreshEditorModelKey}
        disabled={disabled}
        value={config}
        debugLoading={debugLoading}
        fieldPath="config"
        resultsClassName="detail-page-debug-results-wrapper"
        editorHeight={editorHeight}
      />
    </div>
  );
}

{
  /* end_aigc */
}
