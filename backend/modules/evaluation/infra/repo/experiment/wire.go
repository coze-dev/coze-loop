// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/google/wire"

	exptck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	exptredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	iredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
)

var ExperimentRepoSet = wire.NewSet(
	NewExptRepo,
	NewExptStatsRepo,
	NewExptAggrResultRepo,
	NewExptItemResultRepo,
	// 中心调度的 run log 派发投影（Queueing/reserved 与 Queueing/none 的读写）
	NewExptItemDispatchRepo,
	exptmysql.NewExptItemDispatchDAO,
	NewExptTurnResultRepo,
	NewExptItemRefRepo,
	NewExptRunLogRepo,
	NewExptTurnResultFilterRepo,
	NewExptAnnotateRepo,
	NewExptResultExportRecordRepo,
	NewExptInsightAnalysisRecordRepo,
	NewExptTemplateRepo,
	NewQuotaService,
	NewEvalAsyncRepo,
	idem.NewIdempotentService,
	// DAO Sets
	exptmysql.ExperimentMySQLDAOSet,
	exptredis.ExperimentRedisDAOSet,
	exptck.ExperimentCKDAOSet,
	iredis.IdemRedisDAOSet,
)
