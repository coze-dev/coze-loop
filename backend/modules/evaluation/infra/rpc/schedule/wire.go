// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"github.com/google/wire"
)

var ExptScheduleRPCSet = wire.NewSet(
	NewNoopExptScheduleAdapter,
)
