// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { featureFlagStorage } from './utils/storage';

export const getFlags = () => {
  const flags = featureFlagStorage.getFlags();
  return flags;
};
