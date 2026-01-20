// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
class Env {
  get isPPE() {
    return IS_PROD;
  }
}

export const runtimeEnv = new Env();
