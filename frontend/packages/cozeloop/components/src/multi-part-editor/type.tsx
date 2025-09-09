// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type Content } from '@cozeloop/api-schema/evaluation';
import {
  type ContentType,
  ItemErrorType,
  type MultiModalSpec,
} from '@cozeloop/api-schema/data';

export enum ImageStatus {
  Loading = 'loading',
  Success = 'success',
  Error = 'error',
}

export const ErrorTypeMap = {
  [ItemErrorType.MismatchSchema]: 'schema 不匹配',
  [ItemErrorType.EmptyData]: '空数据',
  [ItemErrorType.ExceedMaxItemSize]: '单条数据大小超限',
  [ItemErrorType.ExceedDatasetCapacity]: '数据集容量超限',
  [ItemErrorType.MalformedFile]: '文件格式错误',
  [ItemErrorType.InternalError]: '系统错误',
  [ItemErrorType.IllegalContent]: '包含非法内容',
  [ItemErrorType.MissingRequiredField]: '缺少必填字段',
  [ItemErrorType.ExceedMaxNestedDepth]: '数据嵌套层数超限',
  [ItemErrorType.TransformItemFailed]: '数据转换失败',
  [ItemErrorType.ExceedMaxImageCount]: '图片数量超限',
  [ItemErrorType.ExceedMaxImageSize]: '图片大小超限',
  [ItemErrorType.GetImageFailed]: '图片获取失败',
  [ItemErrorType.IllegalExtension]: '文件扩展名不合法',
  [ItemErrorType.UploadImageFailed]: '上传图片失败',
};

export interface MultipartItem extends Content {
  uid?: string;
  sourceImage?: {
    status: ImageStatus;
    file?: File;
  };
}

export interface ImageField {
  name?: string;
  url?: string;
  uri?: string;
  thumb_url?: string;
}

export interface UploadAttachmentDetail {
  contentType?: ContentType;
  originImage?: ImageField;
  image?: ImageField;
  errorType?: ItemErrorType;
  errMsg?: string;
}

interface MultiPartContent extends Content {
  uid?: string;
  sourceImage?: {
    status: ImageStatus;
    file?: File;
  };
}

export interface MultipartEditorProps {
  spaceID?: Int64;
  className?: string;
  value?: MultiPartContent[];
  multipartConfig?: MultiModalSpec;
  readonly?: boolean;
  onChange?: (contents: MultiPartContent[]) => void;
  uploadFile?: (params: any) => Promise<string>;
  uploadImageUrl?: (
    urls: string[],
  ) => Promise<UploadAttachmentDetail[] | undefined>;
}
