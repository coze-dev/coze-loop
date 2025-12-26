// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/google/wire"
)

var EvalTargetMetricsSet = wire.NewSet(
	NewEvalTargetMetrics,
)
