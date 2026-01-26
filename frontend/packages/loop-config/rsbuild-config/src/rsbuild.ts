// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-require-imports -- skip */
import { dirname } from 'node:path';

import { pluginSvgr } from '@rsbuild/plugin-svgr';
import { pluginSass } from '@rsbuild/plugin-sass';
import { pluginReact } from '@rsbuild/plugin-react';
import { pluginLess } from '@rsbuild/plugin-less';
import { type RsbuildConfig, mergeRsbuildConfig } from '@rsbuild/core';
import { SemiRspackPlugin } from '@douyinfe/semi-rspack-plugin';
import { GLOBAL_ENVS } from '@coze-studio/bot-env-adapter';
import SubspaceResolvePlugin from '@coze-arch/subspace-resolve-plugin';
import PkgRootWebpackPlugin from '@coze-arch/pkg-root-webpack-plugin';

import { formatDefineVars, getLatestGitCommitHash } from './utils';

// eslint-disable-next-line max-lines-per-function
export function createRsbuildConfig(rsbuildConfig: RsbuildConfig) {
  const defaultConfig: RsbuildConfig = {
    source: {
      include: [/\/node_modules\/marked\//],
      define: formatDefineVars({
        MONACO_UNPKG: 'https://unpkg.com/monaco-editor@0.43.0/min/vs',
        'process.env.IS_PERF_TEST': process.env.IS_PERF_TEST,
        'process.env.BUILD_TYPE': process.env.BUILD_TYPE,
        'process.env.BUILD_BRANCH': process.env.BUILD_BRANCH,
        'process.env.BUILD_VERSION': process.env.BUILD_VERSION,
        'process.env.SCM_BUILD_TYPE': process.env.BUILD_TYPE,
        ...GLOBAL_ENVS,
      }),
      alias: {
        'react-dom': require.resolve('react-dom'),
        react: require.resolve('react'),
        'react-router-dom': require.resolve('react-router-dom'),
        'react-router': require.resolve('react-router'),
        // fix https://github.com/react-dnd/react-dnd/issues/3433
        'react/jsx-runtime.js': 'react/jsx-runtime',
        'react/jsx-dev-runtime.js': 'react/jsx-dev-runtime',
        // redirect i18n
        '@coze-arch/i18n': require.resolve('@cozeloop/i18n-adapter'),
        '@coze-arch/semi-theme-hand01': dirname(
          require.resolve('@coze-arch/semi-theme-hand01/package.json'),
        ),
      },
    },
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
    output: {
      filenameHash: true,
      assetPrefix: '/',
    },
    html: {
      meta: {
        'git-commit-hash': getLatestGitCommitHash(),
      },
    },
    tools: {
      postcss(_config, { addPlugins }) {
        addPlugins(require('tailwindcss/nesting'));
        addPlugins(require('tailwindcss'));
        addPlugins(
          require('@coze-arch/postcss-plugin/ns')({
            prefixSet: [
              {
                regexp: /prismjs\/themes\/.*\.css$/,
                namespace: '.prismjs',
              },
            ],
          }),
        );
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
          new SemiRspackPlugin({ theme: '@coze-arch/semi-theme-hand01' }),
        ]);

        return mergeConfig(config, {
          watchOptions: { poll: true },
        });
      },
    },
    performance: {
      chunkSplit: {
        strategy: 'split-by-experience',
        override: {
          cacheGroups: {
            semiStyles: {
              name: 'semi',
              test: /node_modules\/.*semi.*\.(css|less|sass|scss)/,
              chunks: 'all',
              enforce: true,
            },
            cozeDesign: {
              name: 'lib-coze-design',
              test: /coze-design/,
            },
            semiUI: {
              name: 'lib-semi-ui',
              test: /@douyinfe\/semi-ui/,
            },
            semiFoundation: {
              name: 'lib-semi-foundation',
              test: /@douyinfe\/semi-foundation/,
            },
            mathjax: {
              name: 'lib-mathjax',
              test: /mathjax-full/,
            },
          },
        },
      },
    },
  };

  return mergeRsbuildConfig(defaultConfig, rsbuildConfig);
}
