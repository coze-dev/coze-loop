// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"github.com/google/wire"
)

var LLMRPCSet = wire.NewSet(
	NewLLMRPCProvider,
)
