// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"github.com/google/wire"

	targetmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql"
)

var TargetRepoSet = wire.NewSet(
	NewEvalTargetRepo,
	// DAO Sets
	targetmysql.TargetMySQLDAOSet,
)
