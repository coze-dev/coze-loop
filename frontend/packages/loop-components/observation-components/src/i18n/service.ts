// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
import IntlMessageFormat from 'intl-messageformat';

import type { Locale } from './context';

/**
 * Standalone i18n service that can be used outside React components
 */
export class I18nService {
  private currentLocale: Locale | undefined;

  init(locale: Locale): void {
    this.currentLocale = locale;
  }

  getLocale(): string | undefined {
    return this.currentLocale?.language;
  }

  t(key: string, options?: Record<string, unknown>): string {
    if (!this.currentLocale) {
      return key;
    }

    if (!key) {
      return '';
    }

    let translation = key;
    if (this.currentLocale.locale && key in this.currentLocale.locale) {
      translation = String(this.currentLocale.locale[key]);
    }

    if (options && Object.keys(options).length > 0) {
      try {
        const formatter = new IntlMessageFormat(
          translation,
          this.currentLocale.language,
        );
        return formatter.format(options) as string;
      } catch (error) {
        if (options.currency && options.value !== undefined) {
          return `${options.currency} ${(Number(options.value) || 0).toFixed(2)}`;
        }

        Object.keys(options).forEach(optionKey => {
          if (
            optionKey !== 'currency' &&
            optionKey !== 'value' &&
            optionKey !== 'count'
          ) {
            translation = translation.replace(
              new RegExp(`{{${optionKey}}}`, 'g'),
              String(options[optionKey]),
            );
          }
        });
        return translation;
      }
    }

    return translation;
  }
}

export const i18nService = new I18nService();
