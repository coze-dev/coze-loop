// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { isValidElement, type ReactElement } from 'react';

import { intlClient as I18n } from '../src';

describe('I18n x React', () => {
  I18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {
          error: 'Error: {msg}, {msg2}',
        },
      },
    },
  });

  it('should handle multi params', () => {
    const node = I18n.t('error', { msg: '1', msg2: { msg: '2' } });

    console.info(node);
  });

  it('should parse with ReactNode', () => {
    const node = I18n.t('error', {
      msg: <div>123</div>,
    }) as unknown as ReactElement;

    expect(isValidElement(node)).toBe(true);
  });
});
