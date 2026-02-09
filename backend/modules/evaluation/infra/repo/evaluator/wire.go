// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	evaluatorredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/redis/dao"
	"github.com/google/wire"

	evaluatormysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
)

var EvaluatorRepoSet = wire.NewSet(
	NewEvaluatorRepo,
	NewEvaluatorRecordRepo,
	NewEvaluatorTemplateRepo,
	NewRateLimiterImpl,
	NewPlainRateLimiterImpl,
	// DAO Sets
	evaluatormysql.EvaluatorMySQLDAOSet,
	evaluatorredis.EvaluatorRedisDAOSet,
)
