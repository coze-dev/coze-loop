// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { join } from 'node:path';
import { readFile } from 'node:fs/promises';

import pLimit from 'p-limit';
import walk from 'klaw';
import {
  RushConfiguration,
  type RushConfigurationProject,
} from '@rushstack/rush-sdk';

const rushConfiguration = RushConfiguration.loadFromDefaultLocation({
  startingFolder: process.cwd(),
});
const stringLiteralRegex = /(['"])([^'"\\\r\n]*?)\1/g;
const i18nRegex = /I18n\.t\(\s*(['"])([^'"\\\r\n]+?)\1\s*\)/g;
const concurrency = 8;
const stringLiterals = new Set<string>();
const i18nKeys = new Set<string>();

async function scanFile(fileName: string) {
  // eslint-disable-next-line security/detect-non-literal-fs-filename -- skip
  const content = await readFile(fileName, 'utf-8');
  let match: RegExpExecArray | null;

  do {
    match = stringLiteralRegex.exec(content);
    match && stringLiterals.add(match[2]);
  } while (match !== null);

  do {
    match = i18nRegex.exec(content);
    match && i18nKeys.add(match[2]);
  } while (match !== null);
}

async function scanProject(p: RushConfigurationProject) {
  console.info(`Scan ${p.packageName}`);
  const src = join(p.projectFolder, 'src');
  const walker = walk(src, {
    pathSorter: (pathA, pathB) => pathA.localeCompare(pathB),
  });
  const limit = pLimit(concurrency);
  const tasks: Promise<void>[] = [];
  for await (const item of walker) {
    // skip non-file, d.ts
    if (!item.stats.isFile() || item.path.endsWith('d.ts')) {
      continue;
    }

    // only scan *.ts, *.tsx
    if (item.path.endsWith('.ts') || item.path.endsWith('.tsx')) {
      tasks.push(limit(() => scanFile(item.path)));
    }
  }

  await Promise.allSettled(tasks);
}

async function main() {
  const project = rushConfiguration.getProjectByName('@cozeloop/i18n-adapter');
  stringLiterals.clear();
  i18nKeys.clear();
  const tasks: Promise<void>[] = [];
  const limit = pLimit(concurrency);

  if (project?.consumingProjects.size) {
    console.info('===');
    for (const p of project.consumingProjects.values()) {
      tasks.push(limit(() => scanProject(p)));
    }
  }

  await Promise.allSettled(tasks);

  console.info(i18nKeys.values());
  // console.info(stringLiterals.values());
  console.info(i18nKeys.size);
  // console.info(stringLiterals.size);
}

main();
