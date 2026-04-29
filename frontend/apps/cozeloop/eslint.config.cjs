const { defineConfig } = require('@coze-arch/eslint-config');

module.exports = defineConfig({
  preset: 'web',
  packageRoot: __dirname,
  rules: {
    'no-restricted-imports': 'off',
  },
});
