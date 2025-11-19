// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { execSync } from 'node:child_process';

export function getLatestGitCommitHash(): string {
  if (process.env.NODE_ENV === 'development') {
    return 'local-dev';
  }

  try {
    const gitCommitHash = execSync('git rev-parse HEAD', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'], // stdin: ignore, stdout: pipe, stderr: ignore
    }).trim();
    return gitCommitHash;
  } catch (error) {
    console.info('[getLatestGitCommitHash]', error);
    return '';
  }
}

export function formatDefineVars(vars: Record<string, unknown>) {
  return Object.fromEntries(
    Object.entries(vars).map(([k, v]) => [k, JSON.stringify(v)]),
  );
}
