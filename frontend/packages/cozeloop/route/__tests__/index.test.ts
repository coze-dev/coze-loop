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

import { describe, it, expect } from 'vitest';

import {
  createUseNavigateModule,
  createUseOpenWindow,
  type RouteInfoURLParams,
  type UseRouteInfo,
  type RouteInfo,
} from '../src/index';

describe('index exports', () => {
  it('should export all functions', () => {
    expect(typeof createUseNavigateModule).toBe('function');
    expect(typeof createUseOpenWindow).toBe('function');
  });

  it('should have correct function signatures', () => {
    // Test that createUseNavigateModule returns a function
    const mockUseRouteInfo: UseRouteInfo = () => ({
      app: 'test',
      subModule: 'test',
      detail: 'test',
      baseURL: '/test',
      spaceID: '123',
      getBaseURL: () => '/test',
    });

    const useNavigateModule = createUseNavigateModule(mockUseRouteInfo);
    expect(typeof useNavigateModule).toBe('function');

    const useOpenWindow = createUseOpenWindow(mockUseRouteInfo);
    expect(typeof useOpenWindow).toBe('function');
  });

  it('should export correct types', () => {
    // Type-only test - if this compiles, the types are exported correctly
    const routeParams: RouteInfoURLParams = {
      spaceID: '123',
      enterpriseID: '456',
      organizeID: '789',
    };

    const routeInfo: RouteInfo = {
      getBaseURL: () => '/space/123',
      app: 'test-app',
      subModule: 'test-module',
      detail: 'test-detail',
      spaceID: '123',
      enterpriseID: '456',
      organizeID: '789',
    };

    expect(routeParams).toBeDefined();
    expect(routeInfo).toBeDefined();
  });
});
