// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/// <reference types="@rsbuild/core/types" />

declare module 'process' {
  global {
    namespace NodeJS {
      interface ProcessEnv {
        CDN_INNER_CN: string;
        CDN_PATH_PREFIX: string;
      }
    }
  }
}
