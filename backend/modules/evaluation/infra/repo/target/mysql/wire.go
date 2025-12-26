// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"github.com/google/wire"
)

var TargetMySQLDAOSet = wire.NewSet(
	NewEvalTargetDAO,
	NewEvalTargetRecordDAO,
	NewEvalTargetVersionDAO,
)
