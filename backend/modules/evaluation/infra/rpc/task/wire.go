// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"github.com/google/wire"
)

var TaskRPCSet = wire.NewSet(
	NewTaskRPCAdapter,
)
