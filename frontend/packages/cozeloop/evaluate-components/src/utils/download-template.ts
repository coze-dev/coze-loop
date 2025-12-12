// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import Papa, { type UnparseObject } from 'papaparse';
import { I18n } from '@cozeloop/i18n-adapter';

export const downloadCSVTemplate = () => {
  try {
    const fields = ['input', 'reference_output'];
    const data = [
      [I18n.t('evaluate_biggest_animal_world'), I18n.t('evaluate_blue_whale')],
      [I18n.t('evaluate_living_habits_animal'), I18n.t('eat_fish')],
    ];

    const templateJson: UnparseObject<string[]> = {
      fields,
      data,
    };
    const csv = Papa.unparse(templateJson);
    downloadCsv(csv, 'dataset template');
  } catch (error) {
    console.error(error);
  }
};
export function downloadCsv(csv: string, fileName: string) {
  try {
    const BOM = '\uFEFF';
    const file = new File([BOM, csv], fileName, {
      type: 'text/csv;charset=utf-8',
    });
    const anchor = document.createElement('a');
    anchor.download = fileName;
    anchor.href = URL.createObjectURL(file);
    anchor.click();
  } catch (err) {
    console.error(err);
  }
}
/** @deprecated 后续应该统一使用 `@cozeloop/toolkit 的 fileDownload，收敛代码 */
export const downloadWithUrl = async (
  src: string,
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
) => {
  try {
    const url = preFetch
      ? await fetch(src)
          .then(resp => (resp.ok ? resp.blob() : Promise.reject(resp)))
          .then(blob => URL.createObjectURL(blob))
          .catch(() => undefined)
      : src;
    if (url) {
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      link.click();
      URL.revokeObjectURL(url);
      link.remove();
    }
  } catch (error) {
    console.error(error);
  }
};
