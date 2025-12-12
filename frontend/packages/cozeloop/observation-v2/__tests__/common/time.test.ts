import { describe, it, expect } from 'vitest';

import { formatTime } from '../../src/shared/utils/time';
import {
  SECOND_MS,
  MINUTE_MS,
  HOUR_MS,
  DAY_MS,
} from '../../src/shared/constants';

describe('time utils', () => {
  describe('formatTime', () => {
    it('should format time in milliseconds correctly', () => {
      expect(formatTime(500)).toBe('500ms');
      expect(formatTime(999)).toBe('999ms');
    });

    it('should format time in seconds correctly', () => {
      expect(formatTime(SECOND_MS)).toBe('1.00s');
      expect(formatTime(SECOND_MS * 2.5)).toBe('2.50s');
      expect(formatTime(MINUTE_MS - 1)).toBe('60.00s');
    });

    it('should format time in minutes correctly', () => {
      expect(formatTime(MINUTE_MS)).toBe('1.00min');
      expect(formatTime(MINUTE_MS * 2.5)).toBe('2.50min');
      expect(formatTime(HOUR_MS - 1)).toBe('60.00min');
    });

    it('should format time in hours correctly', () => {
      expect(formatTime(HOUR_MS)).toBe('1.00h');
      expect(formatTime(HOUR_MS * 2.5)).toBe('2.50h');
      expect(formatTime(DAY_MS - 1)).toBe('24.00h');
    });

    it('should format time in days correctly', () => {
      expect(formatTime(DAY_MS)).toBe('1.00d');
      expect(formatTime(DAY_MS * 2.5)).toBe('2.50d');
      expect(formatTime(DAY_MS * 100)).toBe('100.00d');
    });
  });
});
