// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { icu2Type } from '../scripts/utils';

describe('Parse ICU format', () => {
  it('should parse plain text', () => {
    const typeInfos = icu2Type('Click to add a personal access token');

    expect(typeInfos).toMatchObject([]);
  });

  it('should parse interpolation', () => {
    const typeInfos = icu2Type('Fail to fetch metadata: {msg}, {msg2}');

    expect(typeInfos).toMatchObject([
      { key: 'msg', type: 'string' },
      { key: 'msg2', type: 'string' },
    ]);
  });

  it('should parse select', () => {
    const typeInfos = icu2Type(`{gender, select,
  male {He will respond shortly.}
  female {She will respond shortly.}
  other {They will respond shortly.}
}`);

    expect(typeInfos[0]).toMatchObject({
      key: 'gender',
      type: "'male' | 'female' | undefined",
    });
  });

  it('should parse plural', () => {
    const typeInfos = icu2Type(
      '{num, plural, one {# day ({date})} other {# days ({date})}}',
    );

    expect(typeInfos).toMatchObject([
      { key: 'num', type: 'number' },
      { key: 'date', type: 'string' },
    ]);
  });
});
