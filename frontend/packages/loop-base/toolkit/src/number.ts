// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
/* eslint-disable security/detect-unsafe-regex */
/* eslint-disable @typescript-eslint/no-magic-numbers */
export const formatNumberWithCommas = (number?: string | number) =>
  number ? number.toString().replace(/(\d)(?=(?:\d{3})+$)/g, '$1,') : number;

export const formatNumberInThousands = (number: string | number) => {
  const num = typeof number === 'string' ? parseFloat(number) : number;
  const formatted = (num / 1000).toFixed(2);
  const integer = formatted.split('.')[0];
  return `${formatNumberWithCommas(integer)}.${formatted.split('.')[1]}`;
};

export const formatNumberInMillions = (number: string | number) => {
  const num = typeof number === 'string' ? parseFloat(number) : number;
  const formatted = (num / 1000000).toFixed(2);
  const integer = formatted.split('.')[0];
  return `${formatNumberWithCommas(integer)}.${formatted.split('.')[1]}`;
};
