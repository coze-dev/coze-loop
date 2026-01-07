// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { EvaluatorType } from '@cozeloop/api-schema/evaluation';
import { IconCozCode, IconCozAiFill } from '@coze-arch/coze-design/icons';

export const getEvaluatorIcon = (type: EvaluatorType) => {
  if (type === EvaluatorType.Code) {
    return <IconCozCode color="var(--coz-fg-secondary)" />;
  }
  if (type === EvaluatorType.Prompt) {
    return <IconCozAiFill color="var(--coz-fg-secondary)" />;
  }
  return null;
};
