import { describe, it, expect } from 'vitest';

import {
  formatNumberWithCommas,
  formatNumberInThousands,
  formatNumberInMillions,
  formatTimestampToString,
} from '../../src/shared/utils/format';

describe('format utils', () => {
  describe('formatNumberWithCommas', () => {
    it('should format number with commas correctly', () => {
      expect(formatNumberWithCommas(1234567)).toBe('1,234,567');
      expect(formatNumberWithCommas('1234567')).toBe('1,234,567');
      expect(formatNumberWithCommas(1234.567)).toBe('1234.567');
      expect(formatNumberWithCommas('1234.567')).toBe('1234.567');
      expect(formatNumberWithCommas(0)).toBe(0);
      expect(formatNumberWithCommas('0')).toBe('0');
      expect(formatNumberWithCommas(undefined)).toBe(undefined);
    });
  });

  describe('formatNumberInThousands', () => {
    it('should format number in thousands correctly', () => {
      expect(formatNumberInThousands(1234567)).toBe('1,234.57');
      expect(formatNumberInThousands('1234567')).toBe('1,234.57');
      expect(formatNumberInThousands(1000)).toBe('1.00');
      expect(formatNumberInThousands('1000')).toBe('1.00');
    });
  });

  describe('formatNumberInMillions', () => {
    it('should format number in millions correctly', () => {
      expect(formatNumberInMillions(1234567)).toBe('1.23');
      expect(formatNumberInMillions('1234567')).toBe('1.23');
      expect(formatNumberInMillions(1000000)).toBe('1.00');
      expect(formatNumberInMillions('1000000')).toBe('1.00');
    });
  });

  describe('formatTimestampToString', () => {
    it('should format timestamp to string correctly', () => {
      // Unix timestamp (10 digits)
      expect(formatTimestampToString(1700000000)).toMatch(
        /\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/,
      );
      expect(formatTimestampToString('1700000000')).toMatch(
        /\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/,
      );

      // Millisecond timestamp (13 digits)
      expect(formatTimestampToString(1700000000000)).toMatch(
        /\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/,
      );
      expect(formatTimestampToString('1700000000000')).toMatch(
        /\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/,
      );

      // Invalid timestamp
      expect(formatTimestampToString('invalid')).toBe('-');
    });
  });
});
