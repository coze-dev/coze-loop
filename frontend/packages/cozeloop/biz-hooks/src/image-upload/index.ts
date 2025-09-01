import {
  type ContentType,
  type ItemErrorType,
} from '@cozeloop/api-schema/data';

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

export const useImageUrlUpload = () => {
  const uploadImageUrl = async (
    urls: string[],
  ): Promise<UploadAttachmentDetail[] | undefined> =>
    Promise.resolve(undefined);
  return { uploadImageUrl };
};
