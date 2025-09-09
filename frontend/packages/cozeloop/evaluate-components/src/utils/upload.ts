import { utils as XLSXUtils, read as XLSXRead } from 'xlsx';
import Papa from 'papaparse';
import JSZip from 'jszip';
import { FileFormat } from '@cozeloop/api-schema/data';

export const CSV_FILE_NAME = 'index.csv';

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
    reader.onload = function (e) {
      try {
        const text = e.target?.result as ArrayBuffer;
        const data = new Uint8Array(text);
        // 使用更小的内存占用读取
        const workbook = XLSXRead(data, {
          type: 'array',
          // bookSheets: true, // 只读取工作表信息
          // bookProps: true, // 只读取工作簿属性
          sheetRows: 1, // 只读取第一行
        });

        // 获取第一个工作表名称
        const firstSheetName = workbook.SheetNames[0];
        const worksheet = workbook.Sheets[firstSheetName];

        // 获取表头
        const headers = getHeadersFromWorksheet(worksheet);

        resolve(headers);
      } catch (error) {
        reject(error);
      }
    };

    reader.onerror = () => reject(new Error('文件读取失败'));
    reader.readAsArrayBuffer(file);
  });
}

function getHeadersFromWorksheet(worksheet) {
  if (!worksheet['!ref']) {
    return [];
  }

  const range = XLSXUtils.decode_range(worksheet['!ref']);
  const headers: string[] = [];

  // 只读取第一行
  for (let col = range.s.c; col <= range.e.c; col++) {
    const cellAddress = XLSXUtils.encode_cell({ r: 0, c: col });
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
    return { headers: [], error: '文件格式错误' };
  }
};
