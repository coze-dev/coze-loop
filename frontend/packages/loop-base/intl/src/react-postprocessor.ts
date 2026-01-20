// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import {
  type ReactNode,
  isValidElement,
  Fragment,
  createElement,
  cloneElement,
} from 'react';

import { type PostProcessorModule, type TOptions } from 'i18next';

import { stringifyVal } from './utils';

export class ReactPostprocessor implements PostProcessorModule {
  readonly type = 'postProcessor';
  readonly name = 'ReactPostprocessor';
  static processorName = 'ReactPostprocessor';

  process(
    value: string | unknown[],
    key: string | string[],
    options: TOptions,
  ): string {
    // won't happen, only in case
    if (!value || !value.length) {
      return `${options.defaultValue || key}`;
    }

    if (Array.isArray(value)) {
      const v = value as ReactNode[];
      const hasReactElements = v.some(it => isValidElement(it));

      if (!hasReactElements) {
        return v.map(it => stringifyVal(it)).join('');
      }

      return createElement(
        Fragment,
        null,
        ...v.map((it, index) =>
          isValidElement(it)
            ? cloneElement(it, { key: it.key ?? index, ...it.props })
            : stringifyVal(it),
        ),
      ) as unknown as string;
    }

    return value;
  }
}
