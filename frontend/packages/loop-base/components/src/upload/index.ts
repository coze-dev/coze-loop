// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type customRequestArgs } from '@coze-arch/coze-design';

export function uploadFile({
  file,
  fileType = 'image',
  onProgress,
  onSuccess,
  onError,
  spaceID,
}: {
  file: File;
  fileType?: 'image' | 'object';
  onProgress?: customRequestArgs['onProgress'];
  onSuccess?: customRequestArgs['onSuccess'];
  onError?: customRequestArgs['onError'];
  spaceID: string;
}) {
  const result = new Promise<string>((resolve, reject) => {
    (async function () {
      try {
        const formData = new FormData();
        formData.append('file', file);
        formData.append('business_type', 'evaluation');
        formData.append('workspace_id', spaceID);
        const res = await fetch('/api/foundation/v1/files', {
          method: 'POST',
          body: formData,
        });
        const resObj = await res.json();
        onSuccess?.({ status: 200, Uri: resObj?.data?.file_name });
        resolve(resObj?.data?.file_name ?? '');
      } catch (e) {
        onError?.({ status: 500 });
        reject(e);
      }
    })();
  });
  return result;
}
