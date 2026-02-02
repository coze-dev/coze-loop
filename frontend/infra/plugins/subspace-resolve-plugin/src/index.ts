import fs from 'fs';

import { type Compiler } from 'webpack';

export interface SubspaceResolveOptions {
  currSubspace: string;
  exclude?: string[];
  // 是否包括相对路径（仅单测用）
  includeRelativePath?: boolean;
}

class SubspaceResolvePlugin {
  options: SubspaceResolveOptions;

  logList: string[] = [];
  moduleResourceMap: {
    [moduleName: string]: string[];
  } = {};

  constructor(options: SubspaceResolveOptions) {
    this.options = options;
  }

  apply(compiler: Compiler) {
    compiler.hooks.normalModuleFactory.tap(
      'SubspaceResolvePlugin',
      normalModuleFactory => {
        normalModuleFactory.hooks.afterResolve.tapAsync(
          'SubspaceResolvePlugin',
          (resolveData, callback: () => void) => {
            const { resource } = resolveData.createData;
            const { request } = resolveData;

            const isRelativePath =
              request.startsWith('.') || request.startsWith('/');

            // 检测import的对象是不是一个module（而不是一个相对或绝对路径）
            if (
              !resource ||
              (isRelativePath && !this.options.includeRelativePath)
            ) {
              return callback();
            }

            // 如果在exclude里，忽略
            if (this.options.exclude?.includes(request)) {
              return callback();
            }

            this.addResourceMap(request, resource);

            // 是否来自其他子空间的依赖，如果不是，则不做处理
            if (!this.checkIsOtherSubspaceDep(resource)) {
              return callback();
            }

            // 是否当前子空间有该module的依赖
            const resourceInCurrSubspace = resource.replace(
              /common\/temp\/[^/]*/,
              `common/temp/${this.options.currSubspace}`,
            );
            if (fs.existsSync(resourceInCurrSubspace)) {
              resolveData.createData.resource = resourceInCurrSubspace;
              this.logList.push(
                `${request} 重定向: ${resource} => ${resourceInCurrSubspace}`,
              );
            } else {
              this.logList.push(
                `${request} 在子空间${this.options.currSubspace}不存在同版本且同peerDependencies的依赖`,
              );
            }

            callback();
          },
        );
      },
    );
    compiler.hooks.afterEmit.tap('SubspaceResolvePlugin', () => {
      console.log('[SubspaceResolvePlugin]======');
      console.log(Array.from(new Set(this.logList)).join('\n'));
      console.log('=============================');
    });
  }

  checkIsOtherSubspaceDep(resource: string) {
    return (
      /common\/temp\/[^/]*/.test(resource) &&
      !resource.includes(`common/temp/${this.options.currSubspace}`)
    );
  }

  addResourceMap(moduleName, resource) {
    const resourceList = this.moduleResourceMap[moduleName];
    if (resourceList) {
      if (!resourceList.includes(resource)) {
        resourceList.push(resource);
      }
    } else {
      this.moduleResourceMap[moduleName] = [resource];
    }
  }
}

export default SubspaceResolvePlugin;
