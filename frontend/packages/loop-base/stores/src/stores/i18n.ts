// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { create } from 'zustand';
import { I18n } from '@cozeloop/i18n-adapter';

export type I18nLang = 'zh-CN' | 'en-US';

interface I18nState {
  lng: I18nLang;
}

interface I18nAction {
  setLng: (lng: string) => void;
  toggleLng: () => void;
  snapshot: () => I18nState;
}

export const useI18nStore = create<I18nState & I18nAction>((set, get) => {
  const setLng: I18nAction['setLng'] = (lang: string) => {
    I18n.i18next.changeLanguage(lang, () => set({ lng: lang as I18nLang }));
  };

  return {
    lng: I18n.lang as I18nLang,
    setLng,
    toggleLng: () => {
      const target = get().lng === 'zh-CN' ? 'en-US' : 'zh-CN';

      setLng(target);
    },
    snapshot: () => get(),
  };
});
