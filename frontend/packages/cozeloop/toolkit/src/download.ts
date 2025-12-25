// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
export const fileDownload = async (
  url: string,
  filename: string,
  /**
   * 是否由前端提前下载 url 内容，再将已下载的内容提供给用户二次下载
   *
   * true - 提前下载后再提供给用户下载，能保证下载行为正常，但会增加响应耗时
   *
   * false - 让用户直接下载 url 内容，filename 在跨域时会失效；若目标 url 的 response header 格式不正确，则无法自动触发下载弹窗
   *
   * @default true
   */
  preFetch = true,
): Promise<void> => {
  const src = preFetch
    ? await fetch(url)
        .then(resp => (resp.ok ? resp.blob() : Promise.reject(resp)))
        .then(blob => URL.createObjectURL(blob))
        .catch(() => undefined)
    : url;

  if (!src) {
    return;
  }
  const link = document.createElement('a');
  link.href = src;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(src);
};

export async function downloadImageWithCustomName(
  url: string,
  customName?: string,
) {
  // 提取文件名（从 URL 中匹配 `xxx~tplv-xxx-image.jpeg` 部分）
  const fileNameMatch = url.match(
    /\/([^\/?]+)\.(jpg|jpeg|png|gif|webp)(?=\?|$)/i,
  );
  let fileName = customName ?? 'downloaded-image.jpeg'; // 默认文件名

  if (fileNameMatch && fileNameMatch[1]) {
    fileName = `${fileNameMatch[1]}.${fileNameMatch[2]}`; // 组合文件名和扩展名
  }

  try {
    const response = await fetch(url);
    const blob = await response.blob();
    const blobUrl = URL.createObjectURL(blob);

    const a = document.createElement('a');
    a.href = blobUrl;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(blobUrl);
  } catch (error) {
    console.error('下载失败:', error);
  }
}
