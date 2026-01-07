// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest';
import { renderHook } from '@testing-library/react';

import type { UseRouteInfo } from '../src/types';
import { createUseNavigateModule } from '../src/create-use-navigate-module';

// Mock react-router-dom
vi.mock('react-router-dom', () => ({
  useNavigate: vi.fn(),
}));

import { useNavigate } from 'react-router-dom';
const mockUseNavigate = vi.mocked(useNavigate);

describe('createUseNavigateModule', () => {
  const mockNavigateBase = vi.fn();
  const mockGetBaseURL = vi.fn();
  const mockUseRouteInfo: UseRouteInfo = vi.fn(() => ({
    app: 'test-app',
    subModule: 'test-module',
    detail: 'test-detail',
    baseURL: '/space/123',
    spaceID: '123',
    getBaseURL: mockGetBaseURL,
  }));

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNavigate.mockReturnValue(mockNavigateBase);
    mockGetBaseURL.mockReturnValue('/space/123');
  });

  it('should create useNavigateModule hook', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    expect(typeof useNavigateModule).toBe('function');
  });

  it('should handle number navigation (history back/forward)', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    result.current(-1);

    expect(mockNavigateBase).toHaveBeenCalledWith(-1);
    expect(mockGetBaseURL).not.toHaveBeenCalled();
  });

  it('should handle string navigation', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    result.current('profile');

    expect(mockGetBaseURL).toHaveBeenCalledWith(undefined);
    expect(mockNavigateBase).toHaveBeenCalledWith(
      '/space/123/profile',
      undefined,
    );
  });

  it('should handle string navigation with options', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    const options = { replace: true, params: { spaceID: '456' } };
    mockGetBaseURL.mockReturnValue('/space/456');

    result.current('profile', options);

    expect(mockGetBaseURL).toHaveBeenCalledWith({ spaceID: '456' });
    expect(mockNavigateBase).toHaveBeenCalledWith(
      '/space/456/profile',
      options,
    );
  });

  it('should handle string navigation with leading slash', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    result.current('/profile');

    expect(mockGetBaseURL).toHaveBeenCalledWith(undefined);
    expect(mockNavigateBase).toHaveBeenCalledWith(
      '/space/123/profile',
      undefined,
    );
  });

  it('should handle object navigation', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    const to = {
      pathname: 'profile',
      search: '?tab=settings',
      hash: '#section1',
    };

    result.current(to);

    expect(mockGetBaseURL).toHaveBeenCalledWith(undefined);
    expect(mockNavigateBase).toHaveBeenCalledWith(
      {
        ...to,
        pathname: '/space/123/profile',
      },
      undefined,
    );
  });

  it('should handle object navigation with options', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    const to = {
      pathname: 'profile',
      search: '?tab=settings',
    };
    const options = { replace: true, params: { spaceID: '789' } };
    mockGetBaseURL.mockReturnValue('/space/789');

    result.current(to, options);

    expect(mockGetBaseURL).toHaveBeenCalledWith({ spaceID: '789' });
    expect(mockNavigateBase).toHaveBeenCalledWith(
      {
        ...to,
        pathname: '/space/789/profile',
      },
      options,
    );
  });

  it('should handle object navigation without pathname', () => {
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    const to = {
      search: '?tab=settings',
      hash: '#section1',
    };

    result.current(to);

    expect(mockGetBaseURL).toHaveBeenCalledWith(undefined);
    expect(mockNavigateBase).toHaveBeenCalledWith(
      {
        ...to,
        pathname: '/space/123/',
      },
      undefined,
    );
  });

  it('should handle empty baseURL', () => {
    mockGetBaseURL.mockReturnValue('');
    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    const { result } = renderHook(() => useNavigateModule());

    result.current('profile');

    expect(mockNavigateBase).toHaveBeenCalledWith('profile', undefined);
  });
});
