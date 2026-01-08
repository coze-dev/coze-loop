// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

import { createUseOpenWindow, type UseOpenWindow } from '@cozeloop/route-base';

import { useRouteInfo } from './use-route-info';

export const useOpenWindow: UseOpenWindow = createUseOpenWindow(useRouteInfo);
