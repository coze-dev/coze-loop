// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import dayjs from '@/shared/utils/dayjs';

export class ViewDurationManager {
  private prevTime: number | undefined;
  private durationBuffer: number;
  public status: 'paused' | 'running' | 'finished';

  constructor() {
    this.prevTime = undefined;
    this.durationBuffer = 0;
    this.status = 'finished';
  }

  public reset() {
    this.durationBuffer = 0;
    this.prevTime = dayjs().valueOf();
    this.status = 'running';
  }

  public pause() {
    if (this.prevTime !== undefined) {
      this.durationBuffer =
        this.durationBuffer + dayjs().valueOf() - this.prevTime;
      this.prevTime = undefined;
    }

    this.status = 'paused';
  }

  public start() {
    this.prevTime = dayjs().valueOf();
    this.status = 'running';
  }

  public finish() {
    const duration =
      this.durationBuffer +
      (this.prevTime !== undefined ? dayjs().valueOf() - this.prevTime : 0);
    this.prevTime = undefined;
    this.durationBuffer = 0;
    this.status = 'finished';

    return duration;
  }
}
