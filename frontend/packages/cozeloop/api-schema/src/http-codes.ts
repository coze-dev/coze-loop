// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export enum HttpStatusCode {
  /** 100 Continue */
  Continue = 100,
  /** 101 Switching Protocols */
  SwitchingProtocols = 101,
  /** 200 OK */
  OK = 200,
  /** 201 Created */
  Created = 201,
  /** 202 Accepted */
  Accepted = 202,
  /** 204 No Content */
  NoContent = 204,
  /** 300 Multiple Choices */
  MultipleChoices = 300,
  /** 301 Moved Permanently */
  MovedPermanently = 301,
  /** 302 Found */
  Found = 302,
  /** 304 Not Modified */
  NotModified = 304,
  /** 400 Bad Request */
  BadRequest = 400,
  /** 401 Unauthorized */
  Unauthorized = 401,
  /** 403 Forbidden */
  Forbidden = 403,
  /** 404 Not Found */
  NotFound = 404,
  /** 405 Method Not Allowed */
  MethodNotAllowed = 405,
  /** 408 Request Timeout */
  RequestTimeout = 408,
  /** 409 Conflict */
  Conflict = 409,
  /** 410 Gone */
  Gone = 410,
  /** 429 Too Many Requests */
  TooManyRequests = 429,
  /** 500 Internal Server Error */
  InternalServerError = 500,
  /** 501 Not Implemented */
  NotImplemented = 501,
  /** 502 Bad Gateway */
  BadGateway = 502,
  /** 503 Service Unavailable */
  ServiceUnavailable = 503,
  /** 504 Gateway Timeout */
  GatewayTimeout = 504,
  /** 505 HTTP Version Not Supported */
  HTTPVersionNotSupported = 505,
}
