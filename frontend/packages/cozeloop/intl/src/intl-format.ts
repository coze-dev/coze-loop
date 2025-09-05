// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable max-params -- skip */
import IntlMessageFormat from 'intl-messageformat';
import { type ModuleType, type i18n as I18next, type TOptions } from 'i18next';

import { fillMissingOptions } from './utils';

interface Resolved {
  res: string;
  usedKey: string;
  exactUsedKey: string;
  usedLng: string;
  usedNS: string;
}

export class IntlFormat {
  static type: ModuleType = 'i18nFormat';

  private readonly cache = new Map<string, IntlMessageFormat>();

  init(i18next: I18next): void {
    // init with i18next
    this.cache.clear();
  }

  parse(
    res: string,
    options: TOptions & Record<string, unknown>,
    lng: string,
    ns: string,
    key: string,
    info?: { resolved?: Resolved },
  ) {
    try {
      const message = info?.resolved?.res
        ? res
        : `${options.defaultValue ?? key}`;
      const cacheKey = `${ns}###${key}###${res}`;
      const messageFormat = this.cache.has(cacheKey)
        ? this.cache.get(cacheKey)
        : this.cache
            .set(cacheKey, new IntlMessageFormat(message, lng, undefined, {}))
            .get(cacheKey);

      return messageFormat
        ? messageFormat.format(fillMissingOptions(messageFormat, options))
        : res;
    } catch (e) {
      console.warn(`Failed to parse ${key}, ${e}`);
      return res;
    }
  }

  clearCache() {
    this.cache.clear();
  }
}
