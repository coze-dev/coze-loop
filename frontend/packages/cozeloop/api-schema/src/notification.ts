import { EventEmitter } from 'eventemitter3';
import { logger } from '@coze-arch/logger';

import { HttpStatusCode } from './http-codes';

export interface ApiBizErrorEvent {
  code?: number;
  msg: string;
  url: string;
}
export const $notification = new EventEmitter<{
  apiError: string;
  apiBizError: ApiBizErrorEvent;
}>();

class ApiError extends Error {
  code: string | number;

  constructor(code: string | number, message: string) {
    super(message);
    this.code = code;
    this.name = 'ApiError';
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- skip
export function checkResponseData(uri: string, data: any) {
  logger.info({
    namespace: 'API',
    scope: uri,
    message: '-',
    meta: data,
  });

  if (typeof data.code === 'number' && data.code !== 0) {
    const msg = data.msg || data.message || 'Unknown error';
    throw new ApiError(data.code, msg);
  }
}

export function checkFetchResponse(response: Response) {
  if (
    response.status >= HttpStatusCode.OK &&
    response.status < HttpStatusCode.MultipleChoices
  ) {
    return;
  }

  switch (response.status) {
    case HttpStatusCode.BadRequest:
      throw new Error('BadRequest');
    case HttpStatusCode.Unauthorized:
      throw new Error('AuthenticationError');
    case HttpStatusCode.Forbidden:
      throw new Error('PermissionDeniedError');
    case HttpStatusCode.NotFound:
      throw new Error('NotFound');
    case HttpStatusCode.TooManyRequests:
      throw new Error('RateLimitError');
    case HttpStatusCode.RequestTimeout:
      throw new Error('TimeoutError');
    case HttpStatusCode.BadGateway:
      throw new Error('BadGateway');
    default:
      throw new Error(
        response.status >= HttpStatusCode.InternalServerError
          ? 'InternalServerError'
          : 'NetworkError',
      );
  }
}

export function onClientError(uri: string, e: unknown) {
  const error =
    e instanceof SyntaxError
      ? 'Invalid JSON error'
      : e instanceof Error
        ? e.message
        : 'Unknown error';

  $notification.emit('apiError', error);
}

export function onClientBizError(url: string, error: unknown) {
  $notification.emit('apiBizError', {
    url,
    code: error instanceof ApiError ? Number(error.code) : -1,
    msg: error instanceof Error ? error.message : 'Unknown error',
  });
}
