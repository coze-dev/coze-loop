// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createAPI as apiFactory } from '@coze-arch/idl2ts-runtime';
import { type IMeta } from '@coze-arch/idl2ts-runtime';

import {
  checkResponseData,
  checkFetchResponse,
  onClientError,
  onClientBizError,
} from '../notification';

export interface ApiOption {
  /**
   * error toast config
   * @default false
   */
  disableErrorToast?: boolean;
  /** headers */
  headers?: Record<string, string>;
}

export interface ApiResponse {
  code?: number;
  msg?: string;
}

function getBaseUrl() {
  try {
    return process.env.API_SCHEMA_BASE_URL || '';
    // eslint-disable-next-line @coze-arch/use-error-in-catch -- no-catch
  } catch {
    return '';
  }
}

export function createAPI<
  T extends {},
  K,
  O = ApiOption,
  B extends boolean = false,
>(meta: IMeta, cancelable?: B) {
  return apiFactory<T, K & ApiResponse, O, B>(meta, cancelable, false, {
    config: {
      clientFactory: _meta => async (url, init, options) => {
        const headers = {
          'Agw-Js-Conv': 'str', // RESERVED HEADER FOR SERVER
          ...init.headers,
          ...(options?.headers ?? {}),
        };
        const uri = `${getBaseUrl()}${url}`;
        const opts = { ...init, headers };

        try {
          if (init?.body) {
            opts.body = JSON.stringify(init?.body);
          }
          const resp = await fetch(uri, opts);
          checkFetchResponse(resp);

          const data = await resp.json();
          checkResponseData(uri, data);

          return data;
        } catch (e) {
          options.disableErrorToast || onClientError(uri, e);
          onClientBizError(uri, e);
          throw e;
        }
      },
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- skip
  } as any);
}
