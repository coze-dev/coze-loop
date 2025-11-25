// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"github.com/google/wire"
)

var EvaluatorMySQLDAOSet = wire.NewSet(
	NewEvaluatorDAO,
	NewEvaluatorVersionDAO,
	NewEvaluatorRecordDAO,
	NewEvaluatorTemplateDAO,
	NewEvaluatorTagDAO,
)
