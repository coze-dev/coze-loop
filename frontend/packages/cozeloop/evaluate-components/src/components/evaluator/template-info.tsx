// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useMemo } from 'react';

import { type EvaluatorContent } from '@cozeloop/api-schema/evaluation';

import { parseMessagesVariables } from '@/utils/parse-prompt-variable';

import { PromptVariablesList } from './prompt-variables-list';
import { PromptMessage } from './prompt-message';
import { OutputInfo } from './output-info';

export function TemplateInfo({
  data,
  notTemplate,
}: {
  data?: EvaluatorContent;
  notTemplate?: boolean;
}) {
  const variables = useMemo(() => {
    const messages = data?.prompt_evaluator?.message_list ?? [];
    const newVariables = parseMessagesVariables(messages);
    return newVariables;
  }, [data]);

  return (
    <>
      {notTemplate ? null : (
        <div className="text-[16px] leading-8 font-medium coz-fg-plus mb-3">
          {data?.prompt_evaluator?.prompt_template_name}
        </div>
      )}

      <div className="text-sm font-medium coz-fg-primary mb-2">{'Prompt'}</div>
      {data?.prompt_evaluator?.message_list?.map((m, idx) => (
        <PromptMessage className="mb-2" key={idx} message={m} />
      ))}

      {variables?.length ? (
        <PromptVariablesList className="mb-3" variables={variables} />
      ) : null}
      <div className="h-2" />
      <OutputInfo />
    </>
  );
}
