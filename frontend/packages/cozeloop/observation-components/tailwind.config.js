const { resolve } = require('path');

/**
 *  获取项目的 tailwind content 配置
 * @param projectName 项目名称
 * @returns
 */
export function getTailwindContentByProject(projectName) {
  const cwd = process.cwd();
  return resolve(cwd, 'node_modules', projectName, 'dist/**/*.{js,jsx,css}');
}

module.exports = {
  content: [
    './src/**/*.{js,jsx,ts,tsx}',
    getTailwindContentByProject('@cozeloop/components'),
    getTailwindContentByProject('@coze-arch/coze-design'),
  ],
  // Toggle dark-mode based on .dark class or data-mode="dark"
  darkMode: ['class', '[data-mode="dark"]'],
  theme: {
    extend: {},
  },
  presets: [
    require('@coze-arch/tailwind-config'),
    require('@cozeloop/tailwind-plugin/preset'),
  ],
  plugins: [
    require('@coze-arch/tailwind-config/coze'),
    require('@cozeloop/tailwind-plugin'),
  ],
  corePlugins: {
    preflight: false, // 禁用基础样式
  },
};
