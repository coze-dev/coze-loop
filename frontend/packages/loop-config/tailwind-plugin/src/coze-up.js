// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
// tailwindcss-plugin.js
const plugin = require('tailwindcss/plugin');

// theme colors
const lightModeVariables = require('./light');

// 用于生成 CSS 变量的帮助函数
function generateCssVariables(variables, theme) {
  return Object.entries(variables).reduce((acc, [key, value]) => {
    acc[`--${key}`] = theme ? theme(value) : value;
    return acc;
  }, {});
}

const foreground = {
  'coz-fg-hglt': 'colors.brand.9',
  'coz-fg-hglt-dim': 'colors.brand.5',
  'coz-fg-dim-icon': 'rgba(55, 67, 106, 0.38)',
};

const semanticMiddleground = {
  'coz-mg-hglt-secondary-pressed': 'colors.brand.4',
  'coz-mg-hglt-secondary-hovered': 'colors.brand.3',
  'coz-mg-hglt-secondary': 'colors.brand.2',
  'coz-mg-hglt-plus-pressed': 'colors.brand.11',
  'coz-mg-hglt-plus-hovered': 'colors.brand.10',
  'coz-mg-hglt-plus': 'colors.brand.9',
  'coz-mg-hglt-plus-dim': 'colors.brand.5',
  'coz-mg-hglt-pressed': 'colors.brand.5',
  'coz-mg-hglt-hovered': 'colors.brand.4',
  'coz-mg-hglt': 'colors.brand.3',
};

const stroke = {
  'coz-stroke-hglt': 'colors.brand.9',
  'coz-border-primary': 'borderColor.0',
};

const shadow = {
  'coz-shadow-large': 'boxShadow.large',
  'coz-shadow-normal': 'boxShadow.normal',
  'coz-shadow-small': 'boxShadow.small',
};

// 样式语义化
function generateSemanticVariables(semantics, theme, property) {
  return Object.entries(semantics).map(([key, value]) => ({
    [`.${key}`]: {
      [property]: theme(value),
    },
  }));
}

module.exports = plugin(function ({ addBase, addUtilities, theme }) {
  addBase({
    ':root': generateCssVariables(lightModeVariables),
  });

  addBase({
    ':root': {
      ...generateCssVariables(foreground, theme),
      ...generateCssVariables(stroke, theme),
      ...generateCssVariables(shadow, theme),
    },
  });

  addUtilities([
    ...generateSemanticVariables(foreground, theme, 'color'),
    ...generateSemanticVariables(stroke, theme, 'border-color'),
    ...generateSemanticVariables(shadow, theme, 'box-shadow'),
    ...generateSemanticVariables(
      semanticMiddleground,
      theme,
      'background-color',
    ),
  ]);
});
