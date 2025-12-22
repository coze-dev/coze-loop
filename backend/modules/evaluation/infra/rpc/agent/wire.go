// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"github.com/google/wire"
)

var AgentRPCSet = wire.NewSet(
	NewAgentAdapter,
)
