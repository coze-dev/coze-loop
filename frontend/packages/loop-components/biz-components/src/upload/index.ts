// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { nanoid } from 'nanoid';
import { BusinessType } from '@cozeloop/api-schema/foundation';
import { FoundationApi } from '@cozeloop/api-schema';
import { type customRequestArgs } from '@coze-arch/coze-design';

export function uploadFile({
  file,
  fileType = 'image',
  onProgress,
  onSuccess,
  onError,
  spaceID,
  businessType = BusinessType.Evaluation,
}: {
  file: File;
  fileType?: 'image' | 'object';
  onProgress?: customRequestArgs['onProgress'];
  onSuccess?: customRequestArgs['onSuccess'];
  onError?: customRequestArgs['onError'];
  spaceID: string;
  businessType?: BusinessType;
}) {
  const result = new Promise<string>((resolve, reject) => {
    (async function () {
      try {
        const key = `${spaceID}/${nanoid()}/${file.name}`;
        const res = await FoundationApi.SignUploadFile({
          keys: [key],
          business_type: businessType,
          workspace_id: spaceID,
        });
        const fileUrl = res?.uris?.[0];
        if (!fileUrl) {
          throw new Error('fileUrl is empty');
        }
        await fetch(fileUrl, {
          method: 'PUT',
          body: file,
        });
        onSuccess?.({ status: 200, Uri: key });
        resolve(key ?? '');
      } catch (e) {
        onError?.({ status: 500 });
        reject(e);
      }
    })();
  });
  return result;
}
