// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent_studio

import (
	"github.com/google/wire"
)

var AgentStudioRPCSet = wire.NewSet(
	NewSandboxSchedulerAdapter,
)
