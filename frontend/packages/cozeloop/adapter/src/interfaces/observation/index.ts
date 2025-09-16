// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { type ObservationTracesAdapters } from './traces';
import { type ObservationTasksAdapters } from './tasks';
import { type ObservationMetricsAdapters } from './metrics';

export interface ObservationAdapters {
  'obs.metrics': ObservationMetricsAdapters;
  'obs.tasks': ObservationTasksAdapters;
  'obs.traces': ObservationTracesAdapters;
}
