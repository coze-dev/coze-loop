import dayjs from '@/shared/utils/dayjs';

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

const UNIX_LEN = 10;
const UNIX_LEN2 = 13;

export const formatTimestampToString = (
  timestamp: string | number,
  format = 'YYYY-MM-DD HH:mm:ss',
) => {
  const strLen = `${timestamp}`.length;
  if (strLen === UNIX_LEN) {
    return dayjs.unix(Number(timestamp)).format(format);
  } else if (strLen === UNIX_LEN2) {
    return dayjs(Number(timestamp)).format(format);
  }
  return '-';
};
