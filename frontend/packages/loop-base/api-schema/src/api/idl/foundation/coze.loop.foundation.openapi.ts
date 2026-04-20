// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as extra from './../extra';
export { extra };
import * as base from './../../../base';
export { base };
export interface UploadLoopFileRequest {
  /** file type */
  "Content-Type": string,
  /** binary data */
  body: Blob,
  extra?: extra.Extra,
}
export interface UploadLoopFileResponse {
  code?: number,
  msg?: string,
  data?: FileData,
}
export interface FileData {
  bytes?: string,
  file_name?: string,
}