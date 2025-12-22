// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"github.com/google/wire"
)

var TagRPCSet = wire.NewSet(
	NewTagRPCProvider,
)
