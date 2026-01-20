// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
import { reporter as originReporter } from '@coze-arch/logger';

import { PACKAGE_NAMESPACE } from '../constant';

export const reporter = originReporter.createReporterWithPreset({
  namespace: PACKAGE_NAMESPACE,
});
