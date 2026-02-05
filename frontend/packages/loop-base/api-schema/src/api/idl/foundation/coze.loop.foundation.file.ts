// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import * as coze_loop_foundation_openapi from './coze.loop.foundation.openapi';
export { coze_loop_foundation_openapi };
import * as base from './../../../base';
export { base };
import { createAPI } from './../../config';
export interface FileData {
  bytes?: string,
  file_name?: string,
}
export enum BusinessType {
  Prompt = "prompt",
  Evaluation = "evaluation",
  Observability = "observability",
}
export interface UploadFileRequest {
  /** file type */
  content_type: string,
  /** binary data */
  body: Blob,
  /** binary data */
  business_type?: BusinessType,
}
export interface UploadFileResponse {
  code?: number,
  msg?: string,
  data?: FileData,
}
export interface UploadLoopFileInnerRequest {
  /** file type */
  content_type: string,
  /** binary data */
  body: Blob,
}
export interface UploadLoopFileInnerResponse {
  code?: number,
  msg?: string,
  data?: FileData,
}
export interface SignUploadFileRequest {
  /** file key */
  keys: string[],
  option?: SignFileOption,
  /** binary data */
  business_type?: BusinessType,
  /** workspace id */
  workspace_id?: string,
}
export interface SignFileOption {
  /** TTL(second), default 24h */
  ttl?: string
}
export interface SignUploadFileResponse {
  /** the index corresponding to the keys of request */
  uris?: string[],
  /** the index corresponding to the keys of request */
  sign_heads?: SignHead[],
}
export interface SignHead {
  current_time?: string,
  expired_time?: string,
  session_token?: string,
  access_key_id?: string,
  secret_access_key?: string,
}
export interface SignDownloadFileRequest {
  /** file key */
  keys: string[],
  option?: SignFileOption,
  /** binary data */
  business_type?: BusinessType,
}
export interface SignDownloadFileResponse {
  /** the index corresponding to the keys of request */
  uris?: string[]
}
export interface UploadFileOption {
  /** file name */
  file_name?: string,
  /** custom mimetype -> ext mapping */
  mime_type_ext_mapping?: {
    [key: string | number]: string
  },
}
export interface UploadFileForServerRequest {
  /** file mime type */
  mime_type: string,
  /** file binary data */
  body: Blob,
  /** workspace id */
  workspace_id: string,
  /** upload options */
  option?: UploadFileOption,
}
export interface UploadFileForServerResponse {
  code?: number,
  msg?: string,
  data?: FileData,
}
export const SignUploadFile = /*#__PURE__*/createAPI<SignUploadFileRequest, SignUploadFileResponse>({
  "url": "/api/foundation/v1/sign_upload_files",
  "method": "POST",
  "name": "SignUploadFile",
  "reqType": "SignUploadFileRequest",
  "reqMapping": {
    "body": ["keys", "option", "business_type", "workspace_id"]
  },
  "resType": "SignUploadFileResponse",
  "schemaRoot": "api://schemas/foundation_coze.loop.foundation.file",
  "service": "foundationUpload"
});