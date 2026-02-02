// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
interface CallbackParam<RespType> {
  /** 上一次请求的结果，若为 undefined 则表示这是第一次轮询 */
  prevResp?: RespType;
  resp: RespType;
}

export function startPolling<T>({
  req,
  pollingInterval,
  checkComplete,
  onError,
  onSuccess,
  onPollingComplete,
}: {
  req: () => Promise<T>;
  /** 单位是 ms */
  pollingInterval: number;
  /** 每次接口请求成功后调用，返回 true 则停止轮询 */
  checkComplete: (params: CallbackParam<T>) => boolean;
  /** 每次接口请求成功后调用 */
  onSuccess?: (params: CallbackParam<T>) => void;
  /**
   * 通过判断条件停止轮询时调用，入参是最后一次成功的 resp
   *
   * 注意：
   * 1. 首次调用就 complete 也会触发，可以通过 `first` 参数判断
   * 2. 外部调用 stop 方法时，不会触发
   */
  onPollingComplete?: (param: CallbackParam<T>) => void;
  onError?: (error: unknown) => void;
}) {
  let timeout: NodeJS.Timeout | undefined = undefined;
  /**
   * 是否被外部终止
   *
   * 为了解决 req 请求过程中外部调用 stop 方法，虽然 timeout 被清除，但是 req 的回调仍会被执行的问题
   */
  let stoppedByOutside = false;
  let prevResp: T | undefined = undefined;

  const polling = async () => {
    try {
      const resp = await req();
      if (stoppedByOutside) {
        return;
      }
      onSuccess?.({ prevResp, resp });
      if (checkComplete({ prevResp, resp })) {
        onPollingComplete?.({ prevResp, resp });
        return;
      }
      prevResp = resp;
      timeout = setTimeout(polling, pollingInterval);
    } catch (error) {
      onError?.(error);
      prevResp = undefined;
    }
  };
  polling();
  return {
    stop: () => {
      clearTimeout(timeout);
      stoppedByOutside = true;
      prevResp = undefined;
    },
  };
}
