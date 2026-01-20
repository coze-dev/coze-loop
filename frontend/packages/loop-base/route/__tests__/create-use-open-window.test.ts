// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';

import type { UseRouteInfo } from '../src/types';
import { createUseOpenWindow } from '../src/create-use-open-window';

describe('createUseOpenWindow', () => {
  const mockGetBaseURL = vi.fn();
  const mockUseRouteInfo: UseRouteInfo = vi.fn(() => ({
    app: 'test-app',
    subModule: 'test-module',
    detail: 'test-detail',
    baseURL: '/space/123',
    spaceID: '123',
    getBaseURL: mockGetBaseURL,
  }));

  // Mock window.open
  const mockWindowOpen = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetBaseURL.mockReturnValue('/space/123');
    Object.defineProperty(window, 'open', {
      value: mockWindowOpen,
      writable: true,
    });
  });

  it('should create useOpenWindow hook', () => {
    const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
    expect(typeof useOpenWindow).toBe('function');
  });

  describe('getURL', () => {
    it('should return absolute URL as is', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      const httpUrl = result.current.getURL('http://example.com');
      const httpsUrl = result.current.getURL('https://example.com');

      expect(httpUrl).toBe('http://example.com');
      expect(httpsUrl).toBe('https://example.com');
      expect(mockGetBaseURL).not.toHaveBeenCalled();
    });

    it('should combine relative path with baseURL', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      const url = result.current.getURL('/profile');

      expect(mockGetBaseURL).toHaveBeenCalledWith(undefined);
      expect(url).toBe('/space/123/profile');
    });

    it('should handle path without leading slash', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      const url = result.current.getURL('profile');

      expect(url).toBe('/space/123/profile');
    });

    it('should use custom params for baseURL', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      mockGetBaseURL.mockReturnValue('/space/456');
      const url = result.current.getURL('/profile', { spaceID: '456' });

      expect(mockGetBaseURL).toHaveBeenCalledWith({ spaceID: '456' });
      expect(url).toBe('/space/456/profile');
    });

    it('should handle empty baseURL', () => {
      mockGetBaseURL.mockReturnValue('');
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      const url = result.current.getURL('/profile');

      expect(url).toBe('/profile');
    });
  });

  describe('openBlank', () => {
    it('should open URL in new window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      result.current.openBlank('/profile');

      expect(mockWindowOpen).toHaveBeenCalledWith('/space/123/profile');
    });

    it('should open absolute URL in new window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      result.current.openBlank('https://example.com');

      expect(mockWindowOpen).toHaveBeenCalledWith('https://example.com');
    });

    it('should open URL with custom params in new window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      mockGetBaseURL.mockReturnValue('/space/789');
      result.current.openBlank('/profile', { spaceID: '789' });

      expect(mockGetBaseURL).toHaveBeenCalledWith({ spaceID: '789' });
      expect(mockWindowOpen).toHaveBeenCalledWith('/space/789/profile');
    });
  });

  describe('openSelf', () => {
    it('should open URL in same window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      result.current.openSelf('/profile');

      expect(mockWindowOpen).toHaveBeenCalledWith(
        '/space/123/profile',
        '_self',
      );
    });

    it('should open absolute URL in same window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      result.current.openSelf('https://example.com');

      expect(mockWindowOpen).toHaveBeenCalledWith(
        'https://example.com',
        '_self',
      );
    });

    it('should open URL with custom params in same window', () => {
      const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
      const { result } = renderHook(() => useOpenWindow());

      mockGetBaseURL.mockReturnValue('/space/999');
      result.current.openSelf('/settings', { spaceID: '999' });

      expect(mockGetBaseURL).toHaveBeenCalledWith({ spaceID: '999' });
      expect(mockWindowOpen).toHaveBeenCalledWith(
        '/space/999/settings',
        '_self',
      );
    });
  });

  it('should maintain function references across re-renders', () => {
    const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
    const { result, rerender } = renderHook(() => useOpenWindow());

    const firstRender = {
      openBlank: result.current.openBlank,
      openSelf: result.current.openSelf,
      getURL: result.current.getURL,
    };

    rerender();

    expect(result.current.openBlank).toBe(firstRender.openBlank);
    expect(result.current.openSelf).toBe(firstRender.openSelf);
    expect(result.current.getURL).toBe(firstRender.getURL);
  });
});
