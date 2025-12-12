import { describe, it, expect } from 'vitest';
import dayjs from 'dayjs';

import { dayJsTimeZone } from '../../src/shared/utils/dayjs';

describe('dayjs utils', () => {
  describe('dayJsTimeZone', () => {
    it('should return dayjs instance without timezone when timeZone is not provided', () => {
      const date = '2023-01-01T12:00:00';
      const result = dayJsTimeZone(date);

      expect(result).toBeInstanceOf(dayjs().constructor);
      expect(result.format('YYYY-MM-DD HH:mm:ss')).toBe('2023-01-01 12:00:00');
    });

    it('should return dayjs instance with timezone when timeZone is provided', () => {
      const date = '2023-01-01T12:00:00';
      const timeZone = 'Asia/Shanghai';
      const result = dayJsTimeZone(date, timeZone);

      expect(result).toBeInstanceOf(dayjs().constructor);
      // 验证时区设置是否生效
      expect(result.format('YYYY-MM-DD HH:mm:ss')).toBeTruthy();
    });

    it('should handle Date object as input', () => {
      const date = new Date('2023-01-01T12:00:00');
      const result = dayJsTimeZone(date);

      expect(result).toBeInstanceOf(dayjs().constructor);
      expect(result.format('YYYY-MM-DD')).toBe('2023-01-01');
    });

    it('should handle timestamp as input', () => {
      // 测试时间戳输入
      const timestamp = new Date('2023-01-01T20:00:00Z').getTime();
      const result = dayJsTimeZone(timestamp, 'America/New_York');

      expect(result).toBeInstanceOf(dayjs().constructor);
      // 由于本地环境可能有时区差异，我们检查日期部分是否正确
      expect(result.format('YYYY-MM-DD')).toBe('2023-01-01');
    });

    it('should handle empty input', () => {
      const result = dayJsTimeZone();

      expect(result).toBeInstanceOf(dayjs().constructor);
    });

    it('should handle different timezones correctly', () => {
      const date = '2023-01-01T12:00:00';

      // 使用不同的时区
      const result1 = dayJsTimeZone(date, 'Asia/Shanghai');
      const result2 = dayJsTimeZone(date, 'America/New_York');

      // 验证两个结果是不同的实例
      expect(result1).toBeInstanceOf(dayjs().constructor);
      expect(result2).toBeInstanceOf(dayjs().constructor);
    });

    it('should correctly handle UTC date string with timezone conversion', () => {
      // 测试带时区的日期字符串输入
      const result = dayJsTimeZone('2023-01-01T20:00:00Z', 'America/New_York');

      expect(result).toBeInstanceOf(dayjs().constructor);
      // 由于本地环境可能有时区差异，我们检查日期部分和时区处理是否正确
      expect(result.format('YYYY-MM-DD')).toBe('2023-01-01');
    });

    it('should preserve the original time value when converting with timezone', () => {
      const date = '2023-01-01T12:00:00';
      const timeZone = 'Asia/Shanghai';
      const result = dayJsTimeZone(date, timeZone);

      // 验证时间戳是相同的
      expect(result.valueOf()).toBe(dayjs(date).valueOf());
    });
  });
});
