// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trajectory

import (
	"github.com/google/wire"
)

var TrajectoryRPCSet = wire.NewSet(
	NewAdapter,
)
