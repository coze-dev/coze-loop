// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */

/**
 * 处理并发任务，如果在短时间内，相同任务执行，只执行一次，其他任务同时等待结果即可
 */

interface Config {
  // 缓存的时间，如果设置为-1，则永久缓存结果。
  // 如果请求的时间已经超出缓存，则重新发起一个新的请求。
  duration: number;
}
export class Task {
  private time = 0;
  private taskMap: Record<string, Promise<any>> = {};
  private config: Config;
  // 批次
  private batch = 0;
  constructor(config: Config) {
    this.config = config;
  }
  public async exec<T>(task: () => Promise<T>) {
    if (this.config.duration !== -1) {
      const offset = Date.now() - this.time;
      if (offset >= this.config.duration) {
        this.time = Date.now();
        this.batch++;
      }
    }

    // 当前批次不存在任务
    if (!this.taskMap[this.batch]) {
      this.taskMap[this.batch] = new Promise<T>((resolve, reject) => {
        task().then(resolve).catch(reject);
      });
    }
    const result = await this.taskMap[this.batch];
    return result as T;
  }
}
