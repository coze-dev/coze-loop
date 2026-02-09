// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"github.com/google/wire"
)

var EvaluatorRedisDAOSet = wire.NewSet(
	NewEvaluatorProgressDAO,
)
