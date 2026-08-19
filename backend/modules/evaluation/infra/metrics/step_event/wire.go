// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package step_event

import (
	"github.com/google/wire"
)

var StepEventMetricsSet = wire.NewSet(
	NewStepEventMetrics,
)
