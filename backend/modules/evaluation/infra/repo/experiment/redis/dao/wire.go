// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"github.com/google/wire"

	central "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/central_schedule"
)

var ExperimentRedisDAOSet = wire.NewSet(
	NewQuotaDAO,
	NewEvalAsyncDAO,
	central.NewLedger,
)
