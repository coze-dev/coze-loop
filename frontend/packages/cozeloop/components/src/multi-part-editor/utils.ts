import { type MultiModalSpec } from '@cozeloop/api-schema/data';

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

export const getMultipartConfig = (
  multipartConfig?: MultiModalSpec & { max_part_count?: number },
) => {
  const { max_file_count, max_part_count, max_file_size, supported_formats } =
    multipartConfig || {};
  const maxFileCount = max_file_count
    ? Number(max_file_count)
    : DEFAULT_FILE_COUNT;
  const maxPartCount = max_part_count
    ? Number(max_part_count)
    : DEFAULT_PART_COUNT;
  const trueMaxFileCount =
    maxFileCount > maxPartCount ? maxPartCount : maxFileCount;
  const maxFileSize = max_file_size ? Number(max_file_size) : DEFAULT_FILE_SIZE;
  const supportedFormats = (
    supported_formats?.map(format => `.${format}`) || DEFAULT_SUPPORTED_FORMATS
  ).join(',');
  return {
    maxFileCount: trueMaxFileCount,
    maxPartCount,
    maxFileSize,
    supportedFormats,
  };
};
