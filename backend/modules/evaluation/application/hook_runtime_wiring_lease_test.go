// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	infraHook "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
)

// Only encryption is replaced; real snapshot encoding/decoding remains in the path.
type wiringLeaseProtector struct{}

func (wiringLeaseProtector) Protect(_ context.Context, _ string, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}
func (wiringLeaseProtector) Unprotect(_ context.Context, _ string, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

type wiringLeaseRuns struct {
	coordinatorRuns
	repo.IHookExecutionInitializationRepo
	claims []entity.HookClaimAttemptInput
}

func (r *wiringLeaseRuns) ReadAttempt(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
	panic("unexpected attempt read")
}
func (r *wiringLeaseRuns) ClaimAttempt(_ context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
	r.claims = append(r.claims, in)
	return entity.HookAttemptStoreResult{}, entity.ErrHookStoreConflict
}

func TestHookRuntimeWiringExecutorConsumesPlatformTiming(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      int32
	}{
		{"default", `{}`, 30},
		{"configured", `{"lease_seconds":60,"renew_seconds":20}`, 60},
		{"invalid", `{"lease_seconds":20,"renew_seconds":10}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
			codec := infraHook.NewStorageCodec(wiringLeaseProtector{})
			snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "wiring-lease", CreatedAt: time.Now(),
				Config:  &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), TimeoutSeconds: gptr.Of(int32(180)), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}},
				Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"), Initiator: &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("wiring"), Type: gptr.Of("offline")}, EvalSets: []*spi.HookEvalSetRef{}}})
			require.NoError(t, err)
			protected, err := codec.EncodeSnapshot(context.Background(), "test-key", snapshot)
			require.NoError(t, err)
			runs := &wiringLeaseRuns{coordinatorRuns: coordinatorRuns{run: &entity.HookStoredRun{State: entity.HookRunState{Key: key, Before: entity.HookOperation{Status: entity.HookOperationPending}}, Snapshot: protected,
				Operations: []entity.HookStoredOperation{{HookOperationSeed: entity.HookOperationSeed{OperationID: "before"}, Phase: entity.HookPhaseBefore}}}}}
			config := infraHook.NewRuntimeConfigProvider(wiringLoader{value: tc.raw}, true)
			platform := &HookRuntimePlatform{Scope: "wiring-lease", Codec: codec, Config: config, Wake: &HookRuntimeWake{}}
			stores := NewHookRuntimeStores(struct{ db.Provider }{}, nil, platform)
			stores.Runs = runs
			publisher := struct{ events.ExptEventPublisher }{}
			locker := struct{ lock.ILocker }{}
			results := &service.ExptResultServiceImpl{}
			manager := service.NewExptManager(nil, nil, nil, nil, nil, nil, nil, nil, struct{ repo.QuotaRepo }{}, locker, nil, publisher, nil, nil, struct{ metrics.ExptMetric }{}, nil, nil, nil, nil, nil, nil, struct{ service.ExptAggrResultService }{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			managed, err := service.NewExptManagerWithHookFinalization(manager, service.ExptManagerFinalizationDependencies{Runs: runs, Repository: stores.Finalization, Owners: struct {
				repo.IHookFinalizationOwnerReader
			}{}, ExecutionScope: platform.Scope})
			require.NoError(t, err)
			scheduler := &service.ExptSchedulerImpl{Manager: managed, Publisher: publisher, ResultSvc: results}
			consumer := service.NewExptRecordEvalService(managed, nil, publisher, struct{ repo.IExptItemResultRepo }{}, nil, nil, nil, nil, nil, locker, nil, nil, nil, results, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			app, err := wireHookRuntime(&experimentApplication{manager: managed, ExptSchedulerEvent: scheduler, ExptItemEvalEvent: consumer}, stores,
				struct {
					service.EvaluationSetItemService
				}{}, struct {
					service.EvaluationSetVersionService
				}{}, struct{ service.IEvaluationSetService }{}, struct{ repo.IExptItemRefRepo }{}, struct{ repo.IExptTurnResultRepo }{}, struct{ repo.IExptItemResultRepo }{}, struct{ repo.IExperimentRepo }{}, struct{ idgen.IIDGenerator }{})
			require.NoError(t, err)
			_, err = app.HookRuntimeServices().Worker.deps.Executor.Execute(context.Background(), hook.AttemptExecutionInput{Key: key, OperationID: "before", Phase: entity.HookPhaseBefore, ExecutionScope: "wiring-lease", Owner: "worker", AttemptID: 4})
			if tc.want == 0 {
				require.ErrorIs(t, err, service.ErrHookExecutionUnavailable)
				require.Empty(t, runs.claims)
				return
			}
			require.NoError(t, err)
			require.Len(t, runs.claims, 1)
			require.Equal(t, tc.want, gptr.Indirect(runs.claims[0].LeaseSeconds))
		})
	}
}
