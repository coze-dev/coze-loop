// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import i18next from 'i18next';

import { intlClient as I18n } from '../src';

describe('I18n', () => {
  I18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: {
      en: {
        translation: {
          key:
            'You have {numPhotos, plural, ' +
            '=0 {no photos.}' +
            '=1 {one photo.}' +
            'other {# photos.}}',
          error: 'Error: {msg}',
          moreError: 'Error: {msg1},{msg2}',
          escape: "Hello '{{USERNAME}}",
          empty: '',
        },
      },
      'zh-CN': {
        translation: {
          poem: '游园不值',
        },
      },
    },
  });

  it('should get i18next', () => {
    expect(I18n.i18next).toEqual(i18next);
  });

  it('should set & get lang', async () => {
    expect(I18n.lang).toEqual('en');
    await I18n.setLang('zh-CN');
    expect(I18n.lang).toBe('zh-CN');
    expect(I18n.t('poem')).toBe('游园不值');
  });

  it.only('should interpolate', () => {
    expect(I18n.t('error', { msg: '123' })).toBe('Error: 123');
    expect(I18n.t('escape')).toBe('Hello {{USERNAME}}');
    expect(I18n.t('moreError', { msg1: '123', msg2: '321' })).toBe(
      'Error: 123,321',
    );
    expect(I18n.t('moreError', { msg1: '123' })).toBe('Error: 123,');
    expect(I18n.t('moreError', { msg2: '123' })).toBe('Error: ,123');
  });

  it('should interpolate with object', () => {
    expect(I18n.t('error', { msg: { msg: '123' } })).toBe(
      'Error: {"msg":"123"}',
    );
  });

  it('should parse icu format', () => {
    expect(I18n.t('key', { numPhotos: 0 })).toBe('You have no photos.');
    expect(I18n.t('key', { numPhotos: 1 })).toBe('You have one photo.');
    expect(I18n.t('key', { numPhotos: 2 })).toBe('You have 2 photos.');
  });

  it('should parse non exist key/value with defaultValue', () => {
    expect(I18n.t('empty')).toBe('empty');
    expect(I18n.t('non_exist_key', undefined)).toBe('non_exist_key');
    expect(I18n.t('non_exist_key', undefined, 'fallback')).toBe('fallback');
  });
});
