/*
 * Copyright 2025 
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

import { paddingPath, getPath } from '../src/utils';

describe('utils', () => {
  describe('paddingPath', () => {
    it('should add leading slash if path does not start with slash', () => {
      expect(paddingPath('home')).toBe('/home');
      expect(paddingPath('user/profile')).toBe('/user/profile');
    });

    it('should not add leading slash if path already starts with slash', () => {
      expect(paddingPath('/home')).toBe('/home');
      expect(paddingPath('/user/profile')).toBe('/user/profile');
    });

    it('should handle empty string', () => {
      expect(paddingPath('')).toBe('/');
    });
  });

  describe('getPath', () => {
    let consoleSpy: ReturnType<typeof vi.spyOn>;

    beforeEach(() => {
      consoleSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    });

    afterEach(() => {
      consoleSpy.mockRestore();
    });

    it('should return path as is when baseURL is empty', () => {
      expect(getPath('/home', '')).toBe('/home');
      expect(getPath('home', '')).toBe('home');
    });

    it('should combine baseURL and path correctly', () => {
      expect(getPath('/home', '/space/123')).toBe('/space/123/home');
      expect(getPath('home', '/space/123')).toBe('/space/123/home');
    });

    it('should handle path without leading slash', () => {
      expect(getPath('profile', '/space/123')).toBe('/space/123/profile');
    });

    it('should warn and return path when path already starts with baseURL', () => {
      const result = getPath('/space/123/home', '/space/123');
      expect(result).toBe('/space/123/home');
      expect(consoleSpy).toHaveBeenCalledWith('你可以直接使用home');
    });

    it('should handle baseURL without leading slash', () => {
      expect(getPath('/home', 'space/123')).toBe('space/123/home');
    });

    it('should handle empty path', () => {
      expect(getPath('', '/space/123')).toBe('/space/123/');
    });
  });
});
