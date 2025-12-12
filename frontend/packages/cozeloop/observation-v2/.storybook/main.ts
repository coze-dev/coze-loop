import { StorybookConfig } from 'storybook-react-rsbuild';

const config: StorybookConfig = {
  framework: {
    name: 'storybook-react-rsbuild',
    options: {
      builder: {
        rsbuildConfigPath: '.storybook/rsbuild.config.ts',
      },
    },
  },
  rsbuildFinal: config => {
    return config;
  },
  stories: [
    '../src/**/*.stories.@(js|jsx|ts|tsx|mdx)',
    '../src/stories/**/*.mdx',
    '../src/stories/**/*.stories.tsx',
  ],
  addons: [
    '@storybook/addon-essentials',
    '@storybook/addon-interactions',
    '@storybook/addon-links',
  ],
  docs: {
    autodocs: 'tag',
  },
};

export default config;
