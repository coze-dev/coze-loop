// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type MultipartEditorConfig } from './type';

export const DEFAULT_FILE_SIZE = 20 * 1024 * 1024;
export const DEFAULT_FILE_COUNT = 20;
export const DEFAULT_PART_COUNT = 50;
export const DEFAULT_SUPPORTED_FORMATS = [
  '.jpg',
  '.jpeg',
  '.png',
  '.gif',
  '.bmp',
  '.webp',
];

export const DEFAULT_VIDEO_SUPPORTED_FORMATS = [
  '.mp4',
  '.avi',
  '.mov',
  '.wmv',
  '.flv',
  '.mkv',
];

export const getMultipartConfig = (multipartConfig?: MultipartEditorConfig) => {
  const {
    max_file_count,
    max_part_count,
    max_file_size,
    imageEnabled,
    videoEnabled,
    imageSupportedFormats,
    videoSupportedFormats,
    maxVideoSize,
  } = multipartConfig || {};
  const maxFileCount = max_file_count
    ? Number(max_file_count)
    : DEFAULT_FILE_COUNT;
  const maxPartCount = max_part_count
    ? Number(max_part_count)
    : DEFAULT_PART_COUNT;
  const trueMaxFileCount =
    maxFileCount > maxPartCount ? maxPartCount : maxFileCount;
  const maxFileSize = max_file_size ? Number(max_file_size) : DEFAULT_FILE_SIZE;
  const currentImageSupportedFormats = (
    imageSupportedFormats?.map(format =>
      format.startsWith('.') ? format : `.${format}`,
    ) || DEFAULT_SUPPORTED_FORMATS
  ).join(',');
  const currentVideoSupportedFormats = (
    videoSupportedFormats?.map(format =>
      format.startsWith('.') ? format : `.${format}`,
    ) || DEFAULT_VIDEO_SUPPORTED_FORMATS
  ).join(',');
  return {
    maxFileCount: trueMaxFileCount,
    maxPartCount,
    maxFileSize,
    imageEnabled,
    videoEnabled,
    imageSupportedFormats: currentImageSupportedFormats,
    videoSupportedFormats: currentVideoSupportedFormats,
    maxVideoSize,
  };
};
