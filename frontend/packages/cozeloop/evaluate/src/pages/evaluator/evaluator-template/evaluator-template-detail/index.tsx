// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo, useState } from 'react';

import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import { InfoTooltip } from '@cozeloop/components';
import { RouteBackAction } from '@cozeloop/base-with-adapter-components';
import {
  EvaluatorTagKey,
  EvaluatorType,
  type EvaluatorTemplate,
} from '@cozeloop/api-schema/evaluation';
import { IconCozDocument } from '@coze-arch/coze-design/icons';
import { Button, Tag } from '@coze-arch/coze-design';

import { getEvaluatorTypeText } from '@/utils/evaluator';
import {
  CodeEvaluatorLanguageFE,
  codeEvaluatorLanguageMapReverse,
} from '@/constants';

import { EvaluatorTemplateInfo } from '../evaluator-template-info';
import { EvaluatorTemplateDetailPreview } from '../evaluator-template-detail-preview';
import { EvaluatorTemplateCodeDetail } from './evaluator-template-code-detail';

export function EvaluatorTemplateDetail({
  className,
  template,
  onBack,
  onApply,
}: {
  className?: string;
  template: EvaluatorTemplate;
  onBack: () => void;
  onApply?: (
    template: EvaluatorTemplate,
    options?: { codeLanguageType?: string },
  ) => void;
}) {
  const [codeLang, setCodeLang] = useState<
    CodeEvaluatorLanguageFE | undefined
  >();

  const { user_manual_url } = template.evaluator_info || {};
  const { tags } = template;

  const renderTags = useMemo(() => {
    const tagsObj = { ...(tags?.[I18n.lang] || {}) };
    delete tagsObj[EvaluatorTagKey.Category];
    return Object.values(tagsObj).flat();
  }, [tags]);

  return (
    <div
      className={classNames('px-6 flex flex-col overflow-hidden', className)}
    >
      <div className="flex items-center pt-4 pb-4">
        <RouteBackAction onBack={onBack} />
        <div className="ml-2 text-[18px] coz-fg-plus font-bold">
          {template.name}
        </div>
        <InfoTooltip className="ml-1" content={template.description} />
        <div className="flex items-center ml-4">
          <Tag color="primary">
            {getEvaluatorTypeText(template.evaluator_type)}
          </Tag>
          {renderTags.length > 0 ? (
            <>
              <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
              <div className="flex items-center gap-1">
                {renderTags.map(tag => (
                  <Tag key={tag} color="primary">
                    {tag}
                  </Tag>
                ))}
              </div>
            </>
          ) : null}
          <div className="flex items-center text-sm coz-fg-secondary">
            {user_manual_url ? (
              <>
                <div className="mx-3 h-3 w-0 border-0 border-l border-solid coz-stroke-primary" />
                <div
                  className="flex items-center coz-fg-secondary cursor-pointer"
                  onClick={() => {
                    const documentUrl =
                      template?.evaluator_info?.user_manual_url || '';
                    if (documentUrl) {
                      window.open(documentUrl);
                    }
                  }}
                >
                  <IconCozDocument className="mr-1" />
                  {I18n.t('fornax_test_documentation_center')}
                </div>
              </>
            ) : null}
          </div>
        </div>
        <Button
          className="ml-auto"
          onClick={() => {
            onApply?.(template, {
              codeLanguageType:
                codeEvaluatorLanguageMapReverse[
                  codeLang || CodeEvaluatorLanguageFE.Python
                ],
            });
          }}
        >
          {I18n.t('apply')}
        </Button>
      </div>
      <EvaluatorTemplateInfo
        className="mb-4 !coz-fg-secondary"
        template={template}
      />
      {template?.evaluator_type === EvaluatorType.Code ? (
        <EvaluatorTemplateCodeDetail
          template={template}
          onCodeLangChange={setCodeLang}
        />
      ) : (
        <EvaluatorTemplateDetailPreview
          className="flex-1 overflow-auto pb-4"
          template={template}
        />
      )}
    </div>
  );
}
