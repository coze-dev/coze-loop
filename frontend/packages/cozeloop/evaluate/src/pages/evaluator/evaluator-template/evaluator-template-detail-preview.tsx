// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import classNames from 'classnames';
import { I18n } from '@cozeloop/i18n-adapter';
import {
  EvaluateModelConfigEditor,
  TemplateInfo,
} from '@cozeloop/evaluate-components';
import { Scenario } from '@cozeloop/api-schema/llm-manage';
import { type EvaluatorTemplate } from '@cozeloop/api-schema/evaluation';
import { Form } from '@coze-arch/coze-design';

export function EvaluatorTemplateDetailPreview({
  className,
  template,
}: {
  className?: string;
  template: EvaluatorTemplate;
}) {
  return (
    <div className={classNames('', className)}>
      <div className="mb-3">
        <Form.Label>{I18n.t('model_selection')}</Form.Label>
        {template?.evaluator_content?.prompt_evaluator?.model_config ? (
          <EvaluateModelConfigEditor
            label={I18n.t('model_selection')}
            disabled={true}
            scenario={Scenario.scenario_evaluator}
            value={template?.evaluator_content?.prompt_evaluator?.model_config}
          />
        ) : null}
      </div>
      {template?.evaluator_content?.prompt_evaluator ? (
        <TemplateInfo data={template?.evaluator_content} />
      ) : null}
    </div>
  );
}
