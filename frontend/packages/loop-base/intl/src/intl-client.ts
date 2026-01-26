// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import LanguageDetector, {
  type DetectorOptions,
} from 'i18next-browser-languagedetector';
import i18next, { type InitOptions } from 'i18next';

import { ReactPostprocessor } from './react-postprocessor';
import { IntlFormat } from './intl-format';

export interface IntlClientOptions extends InitOptions {
  /** {@link DetectorOptions} for i18next-browser-languagedetector  */
  detection?: DetectorOptions;
}

export class IntlClient {
  get i18next() {
    return i18next;
  }

  get lang() {
    return i18next.language;
  }

  get language() {
    return i18next.language;
  }

  async setLang(lng: string) {
    await i18next.changeLanguage(lng);
  }

  async init(options: IntlClientOptions) {
    await i18next
      .use(LanguageDetector)
      .use(IntlFormat)
      .use(new ReactPostprocessor())
      .init({
        ...options,
        postProcess: [ReactPostprocessor.processorName],
      });
  }

  t(key: string, defaultValue?: string): string;
  t(
    key: string,
    interpolation?: Record<string, unknown>,
    defaultValue?: string,
  ): string;
  t(
    key: string,
    interpolationOrDefaultValue?: Record<string, unknown> | string,
    defaultValue?: string,
  ): string {
    if (typeof interpolationOrDefaultValue === 'string') {
      return i18next.t(key, { defaultValue: interpolationOrDefaultValue });
    }
    return i18next.t(key, { ...interpolationOrDefaultValue, defaultValue });
  }

  /** i18n unsafely, **TRUST THE KEY :)** */
  unsafeT(
    key: string,
    interpolation?: Record<string, unknown>,
    defaultValue?: string,
  ) {
    try {
      return i18next.t(key, { ...interpolation, defaultValue });
    } catch (e) {
      console.warn('Unsafe translate', e);
      return defaultValue ?? key;
    }
  }
}

export const intlClient = new IntlClient();
