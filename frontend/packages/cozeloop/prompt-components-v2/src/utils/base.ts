// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type Prompt } from '@cozeloop/api-schema/prompt';

import { type ButtonConfigProps } from '@/components/prompt-develop/type';

export function getButtonDisabledFromConfig(
  config?: ButtonConfigProps,
  prompt?: Prompt,
): boolean {
  if (typeof config?.disabled === 'function') {
    return config.disabled({ prompt });
  }
  return config?.disabled ?? false;
}

export function getButtonHiddenFromConfig(
  config?: ButtonConfigProps,
  prompt?: Prompt,
): boolean {
  if (typeof config?.hidden === 'function') {
    return config.hidden({ prompt });
  }
  return config?.hidden ?? false;
}
