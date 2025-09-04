// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { Navigate, Outlet, type RouteObject } from 'react-router-dom';
import { lazy } from 'react';

import { PageNotFound } from '@cozeloop/components';

import { BasicLayout } from '@/components';

import { SpaceRoute } from './space-route';
import { EnterpriseRoute } from './enterprise-route';
import { BaseRoute } from './base-route';

// 业务模块
const Auth = lazy(() => import('@cozeloop/auth-pages'));
const Evaluation = lazy(() => import('@cozeloop/evaluate-pages'));
const Observation = lazy(() => import('@cozeloop/observation-pages'));
const Prompt = lazy(() => import('@cozeloop/prompt-pages'));
const Tag = lazy(() => import('@cozeloop/tag-pages'));

export const routeConfig: RouteObject[] = [
  // 登录鉴权
  {
    path: '/auth/*',
    element: <Auth />,
  },
  {
    path: '/',
    element: <BaseRoute />,
    children: [
      {
        index: true,
        element: <Navigate to="/console" replace />,
      },
      // 主体功能
      {
        path: 'console',
        element: <Outlet />,
        children: [
          {
            index: true,
            element: <EnterpriseRoute index />,
          },
          {
            path: 'enterprise',
            element: <EnterpriseRoute />,
          },
          {
            path: 'enterprise/:enterpriseID',
            element: <BasicLayout />,
            children: [
              {
                index: true,
                element: <SpaceRoute index />,
              },
              {
                path: 'space',
                element: <SpaceRoute />,
              },
              {
                path: 'space/:spaceID',
                element: <Outlet />,
                children: [
                  {
                    index: true,
                    element: <Navigate to="pe" replace />,
                  },
                  {
                    path: 'pe/*',
                    element: <Prompt />,
                  },
                  {
                    path: 'evaluation/*',
                    element: <Evaluation />,
                  },
                  {
                    path: 'observation/*',
                    element: <Observation />,
                  },
                  {
                    path: 'tag/*',
                    element: <Tag />,
                  },
                ],
              },
            ],
          },
        ],
      },
      {
        path: '*',
        element: <PageNotFound />,
      },
    ],
  },
];
