// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { resolve } from 'node:path';

import { RushConfiguration } from '@microsoft/rush-lib';

/**
 *  获取项目的 tailwind content 配置
 * @param projectName 项目名称
 * @returns
 */
export function getTailwindContentByProject(projectName: string) {
  // 获取 Rush 配置
  const rushConfig = RushConfiguration.loadFromDefaultLocation();

  // 根据项目名称查找项目
  const project = rushConfig.getProjectByName(projectName);

  if (!project) {
    const cwd = process.cwd();
    return resolve(cwd, 'node_modules', projectName, 'dist/**/*.{js,jsx,css}');
  }

  return resolve(
    rushConfig.rushJsonFolder,
    project.projectRelativeFolder,
    'src/**/*.{ts,tsx}',
  );
}
