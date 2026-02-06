// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
class Env {
  get region(): string {
    return this.isDEV ? 'local' : 'cn';
  }

  get env() {
    return this.isDEV ? 'local' : 'prod';
  }

  get isOversea() {
    return false;
  }

  get isRelease() {
    return this.env === 'prod';
  }

  get isProd() {
    return this.env === 'prod';
  }

  get isBOE() {
    return this.region.includes('boe');
  }

  get isDEV() {
    return process.env.NODE_ENV === 'development';
  }

  get isCN() {
    return this.region === 'cn';
  }

  get isI18n() {
    return false;
  }

  get isPerfTest() {
    return Boolean(process.env.IS_PERF_TEST);
  }
}

export const envs = new Env();
