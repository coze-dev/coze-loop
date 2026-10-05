// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type hookSchedulerTargetCleaner interface {
	CleanupHookSchedulerTargets(context.Context, entity.HookRunKey, []*entity.EvalTargetRecord, bool) error
}

// Reuse exact execute ownership/mapping/readback; only the zombie EndCmd hint differs.
func (e *EvalTargetServiceImpl) CleanupHookSchedulerTargets(ctx context.Context, key entity.HookRunKey, records []*entity.EvalTargetRecord, zombie bool) error {
	if !zombie || e == nil || hookExecutionNil(e.sandboxSchedulerAdapter) {
		return e.CleanupHookTargetSandboxes(ctx, key, records)
	}
	extra := map[string]bool{}
	for _, record := range records {
		if record != nil {
			extra[extractExtraSandboxExecuteID(record)] = true
		}
	}
	copy := *e
	copy.sandboxSchedulerAdapter = hookZombieCleanupAdapter{ISandboxSchedulerAdapter: e.sandboxSchedulerAdapter, extra: extra}
	return copy.CleanupHookTargetSandboxes(ctx, key, records)
}

type hookZombieCleanupAdapter struct {
	rpc.ISandboxSchedulerAdapter
	extra map[string]bool
}

func (a hookZombieCleanupAdapter) Destroy(ctx context.Context, in *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
	copy := *in
	copy.ZombieTimeout = len(in.ExecuteIDs) == 1 && !a.extra[in.ExecuteIDs[0]]
	return a.ISandboxSchedulerAdapter.Destroy(ctx, &copy)
}
