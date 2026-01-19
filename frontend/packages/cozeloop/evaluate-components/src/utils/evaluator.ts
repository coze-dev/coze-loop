// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import React from 'react';

import { I18n } from '@cozeloop/i18n-adapter';
import { type Evaluator, EvaluatorType } from '@cozeloop/api-schema/evaluation';
import { IconCozAi, IconCozCode } from '@coze-arch/coze-design/icons';

export function getEvaluatorIcon(
  evaluatorType: EvaluatorType,
  tags?: Evaluator['tags'],
) {
  if (!tags) {
    switch (evaluatorType) {
      case EvaluatorType.Prompt:
        return React.createElement(IconCozAi);
      case EvaluatorType.Code:
        return React.createElement(IconCozCode);
      default:
        return null;
    }
  } else {
    const { lang } = I18n;
    const tagsMap = tags[lang] || tags['zh-CN'];

    const typeTag = tagsMap.Category?.[0] || 'LLM';
    switch (typeTag) {
      case 'LLM':
        return React.createElement(IconCozAi);
      case 'Code':
        return React.createElement(IconCozCode);
      default:
        return null;
    }
  }
}
