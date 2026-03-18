// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"github.com/google/wire"
)

var PipelineRPCSet = wire.NewSet(
	NewNoopPipelineListAdapter,
)
