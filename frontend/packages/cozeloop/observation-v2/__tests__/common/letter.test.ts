import { describe, it, expect } from 'vitest';

import { capitalizeFirstLetter } from '../../src/shared/utils/letter';

describe('letter utils', () => {
  describe('capitalizeFirstLetter', () => {
    it('should capitalize first letter and lowercase the rest', () => {
      expect(capitalizeFirstLetter('hello')).toBe('Hello');
      expect(capitalizeFirstLetter('HELLO')).toBe('Hello');
      expect(capitalizeFirstLetter('Hello World')).toBe('Hello world');
    });

    it('should handle single character string', () => {
      expect(capitalizeFirstLetter('a')).toBe('A');
      expect(capitalizeFirstLetter('Z')).toBe('Z');
    });

    it('should handle empty string', () => {
      expect(capitalizeFirstLetter('')).toBe('');
    });

    it('should handle strings with special characters', () => {
      expect(capitalizeFirstLetter('123abc')).toBe('123abc'); // 首字符是数字，保持不变
      expect(capitalizeFirstLetter('!hello')).toBe('!hello'); // 首字符是特殊字符，保持不变
      expect(capitalizeFirstLetter('@World')).toBe('@world'); // 首字符是特殊字符，后面的字符小写
    });

    it('should handle strings with numbers and letters', () => {
      expect(capitalizeFirstLetter('test123')).toBe('Test123');
      expect(capitalizeFirstLetter('TEST123')).toBe('Test123');
    });

    it('should handle strings with mixed case', () => {
      expect(capitalizeFirstLetter('mIxEdCaSe')).toBe('Mixedcase');
      expect(capitalizeFirstLetter('camelCase')).toBe('Camelcase');
      expect(capitalizeFirstLetter('PascalCase')).toBe('Pascalcase');
    });

    it('should handle non-ASCII characters', () => {
      // 测试带有重音符号的字符
      expect(capitalizeFirstLetter('école')).toBe('École');
      expect(capitalizeFirstLetter('ÜBER')).toBe('Über');

      // 测试中文字符（不应该被转换）
      expect(capitalizeFirstLetter('你好')).toBe('你好');
      expect(capitalizeFirstLetter('世界')).toBe('世界');
    });
  });
});
