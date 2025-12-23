// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"github.com/google/wire"
)

var IdemRedisDAOSet = wire.NewSet(
	NewIdemDAO,
)
