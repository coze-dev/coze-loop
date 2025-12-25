// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import Papa from 'papaparse';
import JSZip from 'jszip';
import { I18n } from '@cozeloop/i18n-adapter';
import { FileFormat } from '@cozeloop/api-schema/data';

export const CSV_FILE_NAME = 'index.csv';

// eslint-disable-next-line @typescript-eslint/consistent-type-imports
type XLSXType = typeof import('xlsx');

let xlsxCache: XLSXType | null = null;
export async function getXlsx(): Promise<XLSXType> {
  if (xlsxCache) {
    return xlsxCache;
  }

  xlsxCache = await import('xlsx');
  return xlsxCache;
}

export const getCSVHeaders = (file: File): Promise<string[]> =>
  new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = function (e) {
      const text = e.target?.result as string;
      const lines = text?.split('\n');
      if (lines?.length > 0) {
        Papa.parse(lines[0], {
          header: true,
          skipEmptyLines: true,
          transformHeader(header) {
            return header.trim(); // 去除列名前后的空白
          },
          beforeFirstChunk(chunk) {
            try {
              // 分割第一行（标题行）
              const chunkLines = chunk?.split('\n') || [];
              const headers = chunkLines?.[0]?.split(',');

              // 过滤掉空的和自动生成的列名
              const validHeaders = headers?.filter(
                header =>
                  header?.trim() !== '' && !header?.trim()?.match(/^_\d+$/),
              );

              // 重建第一行
              chunkLines[0] = validHeaders?.join(',');
              return chunkLines.join('\n');
            } catch (error) {
              reject(error);
            }
          },
          preview: 1,
          complete(results) {
            resolve(results.meta.fields?.filter(field => !!field) ?? []);
          },
        });
      }
    };
    reader.readAsText(file.slice(0, 10240)); // 读取文件的前10KB
  });

function getXlsxHeaders(file: File): Promise<string[]> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = async function (e) {
      try {
        const xlsx = await getXlsx();
        const text = e.target?.result as ArrayBuffer;
        const data = new Uint8Array(text);
        // 使用更小的内存占用读取
        const workbook = xlsx.read(data, {
          type: 'array',
          // bookSheets: true, // 只读取工作表信息
          // bookProps: true, // 只读取工作簿属性
          sheetRows: 1, // 只读取第一行
        });

        // 获取第一个工作表名称
        const firstSheetName = workbook.SheetNames[0];
        const worksheet = workbook.Sheets[firstSheetName];

        // 获取表头
        const headers = await getHeadersFromWorksheet(worksheet);

        resolve(headers);
      } catch (error) {
        reject(error);
      }
    };

    reader.onerror = () =>
      reject(new Error(I18n.t('knowledge_file_read_fail')));
    reader.readAsArrayBuffer(file);
  });
}

async function getHeadersFromWorksheet(worksheet) {
  if (!worksheet['!ref']) {
    return [];
  }

  const xlsx = await getXlsx();
  const range = xlsx.utils.decode_range(worksheet['!ref']);
  const headers: string[] = [];

  // 只读取第一行
  for (let col = range.s.c; col <= range.e.c; col++) {
    const cellAddress = xlsx.utils.encode_cell({ r: 0, c: col });
    const cell = worksheet[cellAddress];

    if (cell && cell.v !== undefined) {
      headers.push(cell.v);
    } else {
      headers.push(`Column${col + 1}`);
    }
  }

  return headers;
}
export const getFileType = (fileName?: string) => {
  const extension = fileName?.split('.')?.pop()?.toLowerCase() || '';
  if (extension?.includes('csv')) {
    return FileFormat.CSV;
  }
  if (extension?.includes('zip')) {
    return FileFormat.ZIP;
  }
  if (extension?.includes('xlsx') || extension?.includes('xls')) {
    return FileFormat.XLSX;
  }
  return FileFormat.CSV;
};

export const getFileHeaders = async (
  file: File,
): Promise<{
  headers: string[];
  error?: string;
}> => {
  try {
    const fileType = getFileType(file.name);
    if (fileType === FileFormat.CSV) {
      const headers = await getCSVHeaders(file);
      return { headers };
    }
    if (fileType === FileFormat.ZIP) {
      const res = await JSZip.loadAsync(file);
      const csvFileName = Object.keys(res.files).find(
        fileName =>
          fileName === CSV_FILE_NAME || fileName.endsWith(`/${CSV_FILE_NAME}`),
      );
      const csvZipObject = csvFileName && res.files[csvFileName];
      if (!csvZipObject) {
        throw new Error('no index.csv file in zip');
      }
      const csvFile = await csvZipObject
        .async('blob')
        .then(blob => new File([blob], CSV_FILE_NAME));
      const headers = await getCSVHeaders(csvFile);
      return { headers };
    }
    if (fileType === FileFormat.XLSX) {
      const headers = await getXlsxHeaders(file);
      return { headers };
    }
    return { headers: [] };
  } catch (error) {
    console.error(error);
    return { headers: [], error: I18n.t('file_format_error') };
  }
};
