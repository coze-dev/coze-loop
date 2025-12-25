// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @typescript-eslint/no-explicit-any */
import { type ContentPart } from '@cozeloop/api-schema/prompt';
import {
  type Content,
  type Image,
  type ContentType,
} from '@cozeloop/api-schema/evaluation';
import { ItemErrorType, type MultiModalSpec } from '@cozeloop/api-schema/data';

import { type I18nType } from '@/provider';

export enum ImageStatus {
  Loading = 'loading',
  Success = 'success',
  Error = 'error',
}

export type FileItemStatus =
  | 'success'
  | 'uploadFail'
  | 'validateFail'
  | 'validating'
  | 'uploading'
  | 'wait';

export interface ContentPartLoop extends ContentPart {
  uid?: string;
  status?: FileItemStatus;
}

export const getErrorTypeMap = (i18n: I18nType) => ({
  [ItemErrorType.MismatchSchema]: i18n.t('schema_mismatch'),
  [ItemErrorType.EmptyData]: i18n.t('empty_data'),
  [ItemErrorType.ExceedMaxItemSize]: i18n.t('single_data_size_exceeded'),
  [ItemErrorType.ExceedDatasetCapacity]: i18n.t('dataset_capacity_exceeded'),
  [ItemErrorType.MalformedFile]: i18n.t('file_format_error'),
  [ItemErrorType.InternalError]: i18n.t('system_error'),
  [ItemErrorType.IllegalContent]: i18n.t('contains_illegal_content'),
  [ItemErrorType.MissingRequiredField]: i18n.t('missing_required_field'),
  [ItemErrorType.ExceedMaxNestedDepth]: i18n.t('data_nesting_exceeds_limit'),
  [ItemErrorType.TransformItemFailed]: i18n.t('data_conversion_failed'),
  [ItemErrorType.ExceedMaxImageCount]: i18n.t('exceed_max_image_count'),
  [ItemErrorType.ExceedMaxImageSize]: i18n.t('exceed_max_image_size'),
  [ItemErrorType.GetImageFailed]: i18n.t('get_image_failed'),
  [ItemErrorType.IllegalExtension]: i18n.t('illegal_extension'),
  [ItemErrorType.UploadImageFailed]: i18n.t(
    'cozeloop_open_evaluate_image_upload_failed',
  ),
});

export type MultipartItemContentType = ContentType | 'Video';
export interface MultipartItem extends Omit<Content, 'content_type'> {
  uid?: string;
  sourceImage?: {
    status: ImageStatus;
    file?: File;
  };
  sourceVideo?: {
    status: ImageStatus;
    file?: File;
  };
  video?: Image;
  content_type: MultipartItemContentType;
  config?: {
    image_resolution?: string;
    video_fps?: number;
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

export type MultipartEditorConfig = MultiModalSpec & {
  imageEnabled?: boolean;
  imageSupportedFormats?: string[];
  videoEnabled?: boolean;
  videoSupportedFormats?: string[];
  maxVideoSize?: number;
};

export interface MultipartEditorProps {
  spaceID?: Int64;
  className?: string;
  value?: MultipartItem[];
  multipartConfig?: MultipartEditorConfig;
  readonly?: boolean;
  onChange?: (contents: MultipartItem[]) => void;
  uploadFile?: (params: any) => Promise<string>;
  uploadImageUrl?: (
    urls: string[],
  ) => Promise<UploadAttachmentDetail[] | undefined>;
  imageHidden?: boolean;
  videoHidden?: boolean;
  intranetUrlValidator?: (url: string) => boolean;
}
