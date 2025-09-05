// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable @coze-arch/use-error-in-catch */
import JSONBig from 'json-bigint';

const jsonBigIntToString = JSONBig({ storeAsString: true });
export const safeJsonParse = (value?: string | null) => {
  try {
    return value
      ? JSON.parse(JSON.stringify(jsonBigIntToString.parse(value)))
      : '';
  } catch (error) {
    return '';
  }
};
