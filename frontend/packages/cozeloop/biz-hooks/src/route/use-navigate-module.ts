// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { createUseNavigateModule } from '@cozeloop/route-base';

import { useRouteInfo } from './use-route-info';

export const useNavigateModule = createUseNavigateModule(useRouteInfo);
