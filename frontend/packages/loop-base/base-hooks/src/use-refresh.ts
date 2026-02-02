// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

export function useRefresh() {
  const [refreshFlag, setRefreshFlag] = useState(0);
  const refresh = () => setRefreshFlag(prev => prev + 1);
  return [refreshFlag, refresh] as const;
}
