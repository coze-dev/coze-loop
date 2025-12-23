// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"github.com/google/wire"
)

var OpenAPIMetricsSet = wire.NewSet(
	NewEvaluationOApiMetrics,
)
