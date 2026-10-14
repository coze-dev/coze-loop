// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { describe, expect, it, vi } from 'vitest';

import { getCSVHeaders, getFileHeaders } from '../src/utils/upload';

vi.mock('@cozeloop/i18n-adapter', () => ({
  I18n: { t: (key: string) => key },
}));

vi.mock('@cozeloop/api-schema/data', () => ({
  FileFormat: { JSONL: 1, Parquet: 2, CSV: 3, XLSX: 4, ZIP: 100 },
}));

describe('CSV import headers', () => {
  it.each([
    {
      name: 'ordinary fields',
      csv: 'input,output\nQuestion,Answer',
      expected: ['input', 'output'],
    },
    {
      name: 'a quoted header without a trailing record separator',
      csv: '"input\ncontext",output',
      expected: ['input\ncontext', 'output'],
    },
    {
      name: 'an empty file',
      csv: '',
      expected: [],
    },
    {
      name: 'quoted comma',
      csv: '"input,context",output\nQuestion,Answer',
      expected: ['input,context', 'output'],
    },
    {
      name: 'consecutive commas inside quotes',
      csv: '"input,,context",output\nQuestion,Answer',
      expected: ['input,,context', 'output'],
    },
    {
      name: 'line feed inside quotes',
      csv: '"input\ncontext",output\nQuestion,Answer',
      expected: ['input\ncontext', 'output'],
    },
    {
      name: 'CRLF inside quotes, normalized as in the backend CSV reader',
      csv: '"input\r\ncontext",output\r\nQuestion,Answer',
      expected: ['input\ncontext', 'output'],
    },
    {
      name: 'escaped double quotes and consecutive commas',
      csv: '"input ""quoted"",,context",output\nQuestion,Answer',
      expected: ['input "quoted",,context', 'output'],
    },
    {
      name: 'BOM before a multiline quoted field',
      csv: '\uFEFF"input\ncontext",output\nQuestion,Answer',
      expected: ['input\ncontext', 'output'],
    },
    {
      name: 'blank and generated-looking unquoted fields',
      csv: ', input ,,_1,output,\n,Question,,,Answer,',
      expected: ['input', 'output'],
    },
    {
      name: 'literal quotes in an unquoted field',
      csv: 'input"quote,,_1,output\nQuestion,,,Answer',
      expected: ['input"quote', 'output'],
    },
    {
      name: 'duplicate fields retain parser renaming',
      csv: 'input,input,output\nfirst,second,Answer',
      expected: ['input', 'input_1', 'output'],
    },
    {
      name: 'duplicate generated-looking fields remain filtered',
      csv: '_1,_1,input\nfirst,second,Question',
      expected: ['input'],
    },
    {
      name: 'quoted generated-looking fields remain accepted',
      csv: '_1,"_1",input\nfirst,second,Question',
      expected: ['_1', 'input'],
    },
    {
      name: 'duplicate quoted empty fields retain parser handling',
      csv: '"",input,""\nfirst,Question,last',
      expected: ['input', '_1'],
    },
    {
      name: 'only empty fields',
      csv: ',,\n,,',
      expected: [],
    },
    {
      name: 'only the first logical record is used',
      csv: 'input,output\n"value\nwith,,commas",Answer',
      expected: ['input', 'output'],
    },
  ])('reads $name', async ({ csv, expected }) => {
    const file = new File([csv], 'dataset.csv', { type: 'text/csv' });

    await expect(getCSVHeaders(file)).resolves.toEqual(expected);
  });

  it.each([
    ['"input,,context",output\nQuestion,Answer', 'input,,context'],
    ['"input\ncontext",output\nQuestion,Answer', 'input\ncontext'],
    ['"input\r\ncontext",output\r\nQuestion,Answer', 'input\ncontext'],
  ])(
    'exposes the backend key for a selected import source',
    async (csv, source) => {
      const file = new File([csv], 'dataset.csv', { type: 'text/csv' });
      const { headers, error } = await getFileHeaders(file);
      // Go encoding/csv uses these exact keys, including CRLF normalization.
      const backendRow = { [source]: 'Question', output: 'Answer' };

      expect(error).toBeUndefined();
      expect(headers[0]).toBe(source);
      expect(backendRow[headers[0]]).toBe('Question');
    },
  );
});
