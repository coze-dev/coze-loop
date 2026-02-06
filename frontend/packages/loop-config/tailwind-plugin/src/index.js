// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
module.exports = {
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: 'rgba(var(--coze-up-brand-9), 1)',
          linear: 'rgba(var(--coze-up-brand-5), 0)',
          12: 'rgba(var(--coze-up-brand-12), 1)',
          11: 'rgba(var(--coze-up-brand-11), 1)',
          10: 'rgba(var(--coze-up-brand-10), 1)',
          9: 'rgba(var(--coze-up-brand-9), 1)',
          8: 'rgba(var(--coze-up-brand-8), 1)',
          7: 'rgba(var(--coze-up-brand-7), 1)',
          6: 'rgba(var(--coze-up-brand-6), 1)',
          5: 'rgba(var(--coze-up-brand-5), 1)',
          4: 'rgba(var(--coze-up-brand-4), 1)',
          3: 'rgba(var(--coze-up-brand-3), 1)',
          2: 'rgba(var(--coze-up-brand-2), 1)',
          1: 'rgba(var(--coze-up-brand-1), 1)',
        },
      },
      borderColor: {
        0: 'rgba(var(--coze-up-stroke-primary), 0.12)',
      },
      // 保留
      boxShadow: {
        DEFAULT: '0 8px 28px 0px rgba(var(--coze-up-brand-12), 0.08)',
        small: '0 8px 16px 0 rgba(var(--coze-up-brand-12), 0.06)',
        normal: '0 8px 28px 0px rgba(var(--coze-up-brand-12), 0.08)',
        large: '0 8px 48px 0px rgba(var(--coze-up-brand-12), 0.12)',
      },
      btnBorderRadius: {
        large: 'var(--coze-6)',
        normal: 'var(--coze-6)',
        small: 'var(--coze-5)',
        mini: 'var(--coze-4)',
      },
      inputBorderRadius: {
        large: 'var(--coze-6)',
        normal: 'var(--coze-6)',
        small: 'var(--coze-5)',
      },
      inputHeight: {
        large: 'var(--coze-36)',
        normal: 'var(--coze-32)',
        small: 'var(--coze-28)',
      },
    },
  },
};
