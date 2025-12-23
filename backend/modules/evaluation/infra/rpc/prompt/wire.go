// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"github.com/google/wire"
)

var PromptRPCSet = wire.NewSet(
	NewPromptRPCAdapter,
)
