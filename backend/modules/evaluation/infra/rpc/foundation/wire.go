// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package foundation

import (
	"github.com/google/wire"
)

var FoundationRPCSet = wire.NewSet(
	NewAuthRPCProvider,
	NewUserRPCProvider,
	NewFileRPCProvider,
)
