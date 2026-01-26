import path from 'path';
import fs from 'fs';

import webpack from 'webpack';
import { describe, it, expect } from 'vitest';

import SubspaceResolvePlugin from '../src';
import otherPackageContent from './fixtures/common/temp/other-space/comp';
import currPackageContent from './fixtures/common/temp/curr-space/comp';

describe('SubspaceResolvePlugin', () => {
  const createCompiler = (pluginOptions = {}) => {
    // 创建 webpack 配置
    const compiler = webpack({
      mode: 'development',
      entry: path.resolve(__dirname, 'fixtures/test-webpack-entry.js'),
      output: {
        path: path.resolve(__dirname, 'fixtures/dist'),
        filename: 'index.js',
      },
      plugins: [
        new SubspaceResolvePlugin({
          currSubspace: 'curr-space',
          includeRelativePath: true,
          ...pluginOptions,
        }),
      ],
    });

    return compiler;
  };

  it('应该指向当前subspace', async () => {
    const compiler = createCompiler({});

    await new Promise((resolve, reject) => {
      compiler.run((err, stats) => {
        // 验证是否有解析路径
        const fileContent = fs.readFileSync(
          path.resolve(__dirname, 'fixtures/dist/index.js'),
          'utf-8',
        );
        expect(fileContent).toContain(currPackageContent);

        resolve(stats);
      });
    });
  });

  it('配置了exclude', async () => {
    const compiler = createCompiler({
      exclude: ['./common/temp/other-space/comp'],
    });

    await new Promise((resolve, reject) => {
      compiler.run((err, stats) => {
        // 验证是否有解析路径
        const fileContent = fs.readFileSync(
          path.resolve(__dirname, 'fixtures/dist/index.js'),
          'utf-8',
        );
        expect(fileContent).toContain(otherPackageContent);

        resolve(stats);
      });
    });
  });
});
