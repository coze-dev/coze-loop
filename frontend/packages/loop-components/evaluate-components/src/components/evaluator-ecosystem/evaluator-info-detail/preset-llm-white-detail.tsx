// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { I18n } from '@cozeloop/i18n-adapter';
import { type Evaluator } from '@cozeloop/api-schema/evaluation';

import { TemplateInfo } from '@/components/evaluator/template-info';

import { EvaluatorInfoContent } from './evaluator-info-content';

interface PresetLLMWhiteDetailProps {
  evaluator?: Evaluator;
}

export function PresetLLMWhiteDetail({ evaluator }: PresetLLMWhiteDetailProps) {
  return (
    <div className="w-full h-full flex flex-col overflow-auto">
      <div className="text-sm font-medium coz-fg-primary">
        {I18n.t('application_scene')}
      </div>
      <EvaluatorInfoContent evaluator={evaluator as Evaluator} />
      <TemplateInfo data={evaluator?.current_version?.evaluator_content} />
    </div>
  );
}
