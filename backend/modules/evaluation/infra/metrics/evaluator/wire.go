// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"github.com/google/wire"
)

var EvaluatorMetricsSet = wire.NewSet(
	NewEvaluatorMetrics,
)
