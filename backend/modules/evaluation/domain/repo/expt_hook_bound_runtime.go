// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"

// Execution construction must not accidentally mix bound and legacy writers.
type IHookBoundExecutionOwner interface {
	HookExecutionBinding() (entity.HookRunKey, string, string)
}

type HookBoundRuntimeRepositories struct {
	Runs           IHookRepo
	Initialization IHookExecutionInitializationRepo
	Finalization   IHookFinalizationRepo
	Consumer       IHookBoundConsumerRepo
	Progress       IHookTurnProgressRepo
	Gate           IHookGateRepo
	Source         IHookItemSourceRepo
}
