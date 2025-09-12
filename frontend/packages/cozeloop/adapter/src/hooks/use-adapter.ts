// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { useState } from 'react';

import { type Adapters } from '../interfaces';
import { get } from '../adapter';

export function useAdapter<S extends keyof Adapters>(
  scope: S,
  name: keyof Adapters[S],
) {
  const [adapter] = useState(() => get(scope, name));
  return adapter;
}
