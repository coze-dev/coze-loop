// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type PEPromptsAdapters } from './prompts';
import { type PEPlaygroundAdapters } from './playground';

export interface PEAdapters {
  'pe.prompts': PEPromptsAdapters;
  'pe.playground': PEPlaygroundAdapters;
}
