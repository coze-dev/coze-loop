import { defineConfig } from '@rsbuild/core';
import { pluginLess } from '@rsbuild/plugin-less';
import { pluginReact } from '@rsbuild/plugin-react';
import { pluginSass } from '@rsbuild/plugin-sass';
import { pluginSvgr } from '@rsbuild/plugin-svgr';
import { SemiRspackPlugin } from '@douyinfe/semi-rspack-plugin';
import SubspaceResolvePlugin from '@coze-arch/subspace-resolve-plugin';
import PkgRootWebpackPlugin from '@coze-arch/pkg-root-webpack-plugin';

export default defineConfig({
  plugins: [
    pluginLess(),
    pluginSass({
      sassLoaderOptions: {
        sassOptions: {
          silenceDeprecations: ['mixed-decls', 'import', 'function-units'],
        },
      },
    }),
    pluginReact(),
    pluginSvgr({ mixedImport: true }),
  ],

  tools: {
    postcss(_config, { addPlugins }) {
      addPlugins(require('tailwindcss/nesting'));
      addPlugins(require('tailwindcss'));
    },
    rspack(config, { appendPlugins, mergeConfig }) {
      config.ignoreWarnings = [
        /@douyinfe\/semi/,
        /Critical dependency: the request of a dependency is an expression/,
        ...(config.ignoreWarnings ?? []),
      ];

      config.module ??= {};
      config.module.parser ??= {};
      config.module.parser.javascript ??= {};
      config.module.parser.javascript.exportsPresence = false;

      appendPlugins([
        new PkgRootWebpackPlugin({}),
        new SubspaceResolvePlugin({ currSubspace: 'default' }),
        new SemiRspackPlugin({
          theme: '@ies/semi-theme-hand01',
          include: '~@ies/semi-theme-hand01/scss/local.scss',
        }),
      ]);

      return mergeConfig(config, {
        watchOptions: { poll: true },
      });
    },
  },
});
